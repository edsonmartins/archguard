// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package postgres

import (
	"context"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// tenantExpectedTuples computes the COMPLETE authorization tuple set a tenant SHOULD have
// from the source of truth (M4 Fase F, T-031) — the input the reconciler diffs against the
// projection. It reads the domain tables through the TenantTx (RLS-scoped) and derives:
//
//   - `parent` edges of asset_groups and assets (structural hierarchy);
//   - `owner` from assets, `operator`/`auditor` from access assignments, and
//     `has_active_grant` from active grants — but ONLY when the subject membership is
//     ACTIVE.
//
// The membership-status gate is what makes the reconciler the safety net for stale access:
// a revoked/suspended member's tuples are simply NOT expected, so the reconciler removes
// them — covering paths that bypass the per-membership lifecycle projection (e.g. the
// identity-level bulk cascade, T-030c).
//
// has_active_grant is emitted without its valid_window condition: the reconciler diffs by
// (user, relation, object), so a conditioned tuple in the store matches its expected key
// and is preserved; an expired/absent grant is simply not expected and gets cleaned up.
func tenantExpectedTuples(ctx context.Context, ttx *TenantTx, orgID uuid.UUID) ([]domain.RelationTuple, error) {
	tx := ttx.Tx()
	var tuples []domain.RelationTuple
	asset := func(id string) string { return domain.Qualify(orgID, domain.TypeAsset, id) }
	group := func(id string) string { return domain.Qualify(orgID, domain.TypeAssetGroup, id) }
	member := func(id string) string { return domain.Qualify(orgID, domain.TypeMembership, id) }

	org := orgID.String()

	// 1) asset_group `parent` edges. (INV-5: predicado de tenant explícito.)
	grpRows, err := collectPairs(ctx, tx,
		`SELECT id::text, parent_group_id::text FROM asset_group
		 WHERE organization_id = $1 AND parent_group_id IS NOT NULL`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: asset_group parent: %w", err)
	}
	for _, r := range grpRows {
		tuples = append(tuples, domain.RelationTuple{User: group(r.b), Relation: domain.RelParent, Object: group(r.a)})
	}

	// 2) asset `parent` edges.
	apRows, err := collectPairs(ctx, tx,
		`SELECT id::text, parent_group_id::text FROM asset
		 WHERE organization_id = $1 AND parent_group_id IS NOT NULL`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: asset parent: %w", err)
	}
	for _, r := range apRows {
		tuples = append(tuples, domain.RelationTuple{User: group(r.b), Relation: domain.RelParent, Object: asset(r.a)})
	}

	// 3) asset `owner` — only for ACTIVE owner memberships (both tables tenant-scoped).
	ownRows, err := collectPairs(ctx, tx,
		`SELECT a.id::text, a.owner_membership_id::text
		 FROM asset a JOIN membership m ON m.id = a.owner_membership_id
		 WHERE a.organization_id = $1 AND m.organization_id = $1
		   AND a.owner_membership_id IS NOT NULL AND m.status = 'active'`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: asset owner: %w", err)
	}
	for _, r := range ownRows {
		tuples = append(tuples, domain.RelationTuple{User: member(r.b), Relation: domain.RelOwner, Object: asset(r.a)})
	}

	// 4) operator/auditor from MEMBERSHIP-subject assignments — only for ACTIVE memberships.
	asgRows, err := collectQuads(ctx, tx,
		`SELECT aa.subject_id::text, aa.relation, aa.object_type, aa.object_id::text
		 FROM asset_access_assignment aa JOIN membership m ON m.id = aa.subject_id
		 WHERE aa.organization_id = $1 AND m.organization_id = $1
		   AND aa.subject_type = 'membership' AND m.status = 'active'`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: access assignments (membership): %w", err)
	}
	for _, r := range asgRows {
		obj := domain.Qualify(orgID, domain.ObjectType(r.c), r.d)
		tuples = append(tuples, domain.RelationTuple{User: member(r.a), Relation: r.b, Object: obj})
	}

	// 4b) operator/auditor from GROUP-subject assignments (D1). The subject is the group
	// userset `group:<id>#member`; it is NOT gated on membership status — the group's
	// members are gated by their own `member` tuples (query 6, active-only).
	grpAsgRows, err := collectQuads(ctx, tx,
		`SELECT aa.subject_id::text, aa.relation, aa.object_type, aa.object_id::text
		 FROM asset_access_assignment aa
		 WHERE aa.organization_id = $1 AND aa.subject_type = 'group'`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: access assignments (group): %w", err)
	}
	for _, r := range grpAsgRows {
		obj := domain.Qualify(orgID, domain.ObjectType(r.c), r.d)
		tuples = append(tuples, domain.RelationTuple{User: groupUserset(orgID, r.a), Relation: r.b, Object: obj})
	}

	// 6) `member` edges — only for ACTIVE memberships (a departed member leaves the group).
	memRows, err := collectPairs(ctx, tx,
		`SELECT gm.membership_id::text, gm.group_id::text
		 FROM group_membership gm JOIN membership m ON m.id = gm.membership_id
		 WHERE gm.organization_id = $1 AND m.organization_id = $1 AND m.status = 'active'`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: group memberships: %w", err)
	}
	for _, r := range memRows {
		tuples = append(tuples, domain.RelationTuple{User: member(r.a), Relation: domain.RelMember, Object: domain.Qualify(orgID, domain.TypeGroup, r.b)})
	}

	// 5) has_active_grant from ACTIVE grants on assets — only for ACTIVE subject memberships.
	grantRows, err := collectPairs(ctx, tx,
		`SELECT g.subject_membership_id::text, g.target_id::text
		 FROM privileged_grant g JOIN membership m ON m.id = g.subject_membership_id
		 WHERE g.organization_id = $1 AND m.organization_id = $1
		   AND g.status = 'active' AND g.target_type = 'asset' AND m.status = 'active'`, org)
	if err != nil {
		return nil, fmt.Errorf("expected: active grants: %w", err)
	}
	for _, r := range grantRows {
		tuples = append(tuples, domain.RelationTuple{User: member(r.a), Relation: domain.RelHasActiveGrant, Object: asset(r.b)})
	}

	return tuples, nil
}

// groupUserset is the `group:<id>#member` subject ref (tenant-qualified) — the userset a
// group-subject operator/auditor assignment targets, expanded through `member` tuples (D1).
func groupUserset(orgID uuid.UUID, id string) string {
	return domain.Qualify(orgID, domain.TypeGroup, id) + "#member"
}

type pair struct{ a, b string }
type quad struct{ a, b, c, d string }

// collectPairs runs a two-text-column query and returns the rows, fully drained (so the
// tx is free for the next query — one active result set at a time on pgx).
func collectPairs(ctx context.Context, tx pgx.Tx, q string, args ...any) ([]pair, error) {
	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.a, &p.b); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// collectQuads runs a four-text-column query and returns the rows, fully drained.
func collectQuads(ctx context.Context, tx pgx.Tx, q string, args ...any) ([]quad, error) {
	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []quad
	for rows.Next() {
		var v quad
		if err := rows.Scan(&v.a, &v.b, &v.c, &v.d); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
