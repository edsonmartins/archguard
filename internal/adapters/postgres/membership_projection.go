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

// membershipAccessTupleUpdates gathers the PERSISTENT access tuples a membership holds as
// a subject — `owner` (from assets it owns) and `operator`/`auditor` (from its access
// assignments) — as TupleUpdates with the given presence (write when present, delete
// otherwise). It is the membership-lifecycle projection (M4 Fase E, T-030): on revoke/
// suspend the tuples are DELETED so a departed member keeps no access in the graph; on
// reactivation/activation they are WRITTEN back.
//
// has_active_grant is intentionally NOT included: a revoked membership's privileged grants
// must be CASCADE-REVOKED (Fase B then removes the has_active_grant tuple and the derived
// sessions), not merely hidden from the graph — deleting the tuple while the grant stays
// active would be inconsistent (the reconciler would re-add it). That cascade is a
// separate follow-up.
func membershipAccessTupleUpdates(ctx context.Context, tx pgx.Tx, orgID, membershipID uuid.UUID, present bool) ([]domain.TupleUpdate, error) {
	subjectRef := domain.Qualify(orgID, domain.TypeMembership, membershipID.String())
	op := domain.TupleDelete
	if present {
		op = domain.TupleWrite
	}

	// Collect owned-asset ids first (one result set at a time on the tx).
	assetIDs, err := scanIDs(ctx, tx,
		`SELECT id::text FROM asset WHERE organization_id = $1 AND owner_membership_id = $2`,
		orgID.String(), membershipID.String())
	if err != nil {
		return nil, fmt.Errorf("postgres: leitura de ativos do membership falhou: %w", err)
	}

	// Collect this membership's access assignments.
	type asg struct{ relation, objType, objID string }
	var assignments []asg
	rows, err := tx.Query(ctx,
		`SELECT relation, object_type, object_id::text FROM asset_access_assignment
		 WHERE organization_id = $1 AND subject_type = 'membership' AND subject_id = $2`,
		orgID.String(), membershipID.String())
	if err != nil {
		return nil, fmt.Errorf("postgres: leitura de atribuições do membership falhou: %w", err)
	}
	for rows.Next() {
		var a asg
		if err := rows.Scan(&a.relation, &a.objType, &a.objID); err != nil {
			rows.Close()
			return nil, err
		}
		assignments = append(assignments, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Collect this membership's access-group bindings (`member` edges).
	groupIDs, err := scanIDs(ctx, tx,
		`SELECT group_id::text FROM group_membership WHERE organization_id = $1 AND membership_id = $2`,
		orgID.String(), membershipID.String())
	if err != nil {
		return nil, fmt.Errorf("postgres: leitura de grupos do membership falhou: %w", err)
	}

	var updates []domain.TupleUpdate
	for _, id := range assetIDs {
		updates = append(updates, domain.TupleUpdate{Op: op, Tuple: domain.RelationTuple{
			User:     subjectRef,
			Relation: domain.RelOwner,
			Object:   domain.Qualify(orgID, domain.TypeAsset, id),
		}})
	}
	for _, id := range groupIDs {
		updates = append(updates, domain.TupleUpdate{Op: op, Tuple: domain.RelationTuple{
			User:     subjectRef,
			Relation: domain.RelMember,
			Object:   domain.Qualify(orgID, domain.TypeGroup, id),
		}})
	}
	for _, a := range assignments {
		objRef := domain.Qualify(orgID, domain.ObjectType(a.objType), a.objID)
		upd, err := domain.ProjectRoleAssignment(objRef, a.relation, subjectRef, present)
		if err != nil {
			return nil, err
		}
		updates = append(updates, upd)
	}
	return updates, nil
}

// scanIDs runs a single-column text query and returns the values, closing the rows.
func scanIDs(ctx context.Context, tx pgx.Tx, q string, args ...any) ([]string, error) {
	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// enqueueMembershipLifecycle enqueues the membership's access-tuple deletes (present=false,
// on revoke/suspend) or writes (present=true, on reactivation/activation) in the caller's
// transaction, atomic with the status change (RFC-0004 §4). No-op when the membership has
// no derived tuples.
func enqueueMembershipLifecycle(ctx context.Context, ttx *TenantTx, orgID, membershipID uuid.UUID, present bool) error {
	updates, err := membershipAccessTupleUpdates(ctx, ttx.Tx(), orgID, membershipID, present)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	return NewAuthzOutbox(ttx.Tx()).Enqueue(ctx, updates)
}
