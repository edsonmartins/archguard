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
	"strings"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/jackc/pgx/v5"
)

// PostgresPDP is the ArchGuard in-domain Policy Decision Point backed by the
// authz_tuple projection store (pacote 007, T-010). It implements
// domain.PolicyDecisionPoint by loading the requesting tenant's tuples fresh and
// evaluating the authorization model over them (domain.Evaluate). It is the
// vendor-neutral engine the ADR-0005 port allows; an OpenFGA adapter would slot
// in behind the same interface.
//
// FAIL-CLOSED (INV-6 / RFC-0004 §6): a Check that cannot read the store returns
// domain.ErrPDPUnavailable, and the caller denies; a COMPUTED refusal is
// Decision{Allowed:false}, err==nil. A store read never silently degrades to an
// allow, and a database error is never collapsed into a denial that looks
// computed — the two stay distinct so the audit records `error` vs `denied`.
//
// NO CACHE for the privileged decision (RFC-0004 §5): every Check reads the store
// at decision time. A short cache exists only for listings (T-011), never here.
type PostgresPDP struct {
	db Beginner
}

// NewPostgresPDP builds the PDP over a connection source.
func NewPostgresPDP(db Beginner) *PostgresPDP {
	return &PostgresPDP{db: db}
}

// Check answers one authorization question. Cross-tenant or unqualified refs are a
// computed DENIAL (spec "Consulta cruzada"), decided before any store access. A
// store read failure is domain.ErrPDPUnavailable (fail-closed). Otherwise the
// verdict is domain.Evaluate over the tenant's freshly loaded graph.
func (p *PostgresPDP) Check(ctx context.Context, req domain.CheckRequest) (domain.Decision, error) {
	if !req.Valid() {
		return domain.DenyDecision("requisição malformada"), nil
	}
	if err := domain.GuardSameTenant(req.Tuple.User, req.Tuple.Object); err != nil {
		// A cross-tenant / unqualified request is a policy denial, not an infra error.
		return domain.DenyDecision("cross-tenant: " + err.Error()), nil
	}
	org, _ := domain.RefOrg(req.Tuple.Object) // ok: GuardSameTenant confirmed it
	graph, err := p.loadTenantGraph(ctx, org)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
	}
	return domain.Evaluate(graph, req.Tuple.Object, req.Tuple.Relation, req.Tuple.User, req.Context)
}

// CanOpenPrivilegedSession is the decision entrypoint the privileged-session opener
// calls (RFC-0004 §5): it asks can_open_privileged_session for the membership over
// the asset, with the session's proven ACR in the context, WITHOUT cache. The
// caller opens the session only on Decision.Allowed, and treats any error as a
// denial (fail-closed) — never opening a privileged session when the PDP could not
// decide.
func (p *PostgresPDP) CanOpenPrivilegedSession(ctx context.Context, membershipRef, assetRef string, cc domain.CheckContext) (domain.Decision, error) {
	return p.Check(ctx, domain.CheckRequest{
		Tuple:   domain.RelationTuple{User: membershipRef, Relation: domain.RelCanOpenPrivilegedSession, Object: assetRef},
		Context: cc,
	})
}

// ListObjects returns the tenant-qualified ids of every object of req.Type on which
// req.User holds req.Relation (reverse query; access review, T-014). It enumerates
// the candidate objects of that type in the tenant's store, then evaluates each —
// so inherited and grant-derived access is included, not only direct tuples.
func (p *PostgresPDP) ListObjects(ctx context.Context, req domain.ListObjectsRequest) ([]string, error) {
	if !req.Valid() {
		return nil, fmt.Errorf("pdp: consulta reversa malformada")
	}
	org, ok := domain.RefOrg(req.User)
	if !ok {
		return nil, fmt.Errorf("pdp: usuário sem tenant %q", req.User)
	}
	graph, err := p.loadTenantGraph(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
	}
	candidates, err := p.objectsOfType(ctx, org, req.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
	}
	var out []string
	for _, obj := range candidates {
		dec, err := domain.Evaluate(graph, obj, req.Relation, req.User, domain.CheckContext{})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
		}
		if dec.Allowed {
			out = append(out, obj)
		}
	}
	return out, nil
}

// ReviewAsset lists every membership of the asset's tenant with effective access
// to it, and the origin of each (direct/inherited/by-grant) — the access-review
// campaign query (T-014 / spec "Campanha de revisão"). It loads the tenant graph
// fresh, enumerates the membership candidates present in it, and classifies each
// via domain.ReviewAssetAccess. Fail-closed: a load error surfaces as
// ErrPDPUnavailable, never a silently short review.
func (p *PostgresPDP) ReviewAsset(ctx context.Context, assetRef string) ([]domain.AccessReviewEntry, error) {
	org, ok := domain.RefOrg(assetRef)
	if !ok {
		return nil, fmt.Errorf("pdp: ativo sem tenant %q", assetRef)
	}
	graph, err := p.loadTenantGraph(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
	}
	candidates, err := p.membershipCandidates(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPDPUnavailable, err)
	}
	return domain.ReviewAssetAccess(graph, assetRef, candidates, domain.CheckContext{})
}

// membershipCandidates returns the distinct membership refs that appear as a
// subject anywhere in the tenant's graph — the population a review evaluates.
func (p *PostgresPDP) membershipCandidates(ctx context.Context, org string) ([]string, error) {
	var out []string
	err := WithTx(ctx, p.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT tuple_user FROM authz_tuple
			WHERE organization_id = $1 AND tuple_user LIKE $2`,
			org, "org:"+org+"/"+string(domain.TypeMembership)+":%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err != nil {
				return err
			}
			out = append(out, u)
		}
		return rows.Err()
	})
	return out, err
}

// Write applies tuple updates to the store idempotently (the port surface the
// publisher/bootstrap use). Each tuple is validated (INV-5) before it is applied.
func (p *PostgresPDP) Write(ctx context.Context, updates []domain.TupleUpdate) error {
	return WithTx(ctx, p.db, func(tx pgx.Tx) error {
		for _, u := range updates {
			if !u.Op.Valid() {
				return fmt.Errorf("pdp: operação inválida %q", u.Op)
			}
			if err := domain.ValidateTuple(u.Tuple); err != nil {
				return err
			}
			org, _ := domain.RefOrg(u.Tuple.Object)
			if err := applyToStore(ctx, tx, org, u); err != nil {
				return err
			}
		}
		return nil
	})
}

// Read returns tuples matching the filter (reconciliation/debug). At least one of
// user/object should be set to bound the scan to a tenant.
func (p *PostgresPDP) Read(ctx context.Context, filter domain.TupleFilter) ([]domain.RelationTuple, error) {
	conds := make([]string, 0, 3)
	args := make([]any, 0, 3)
	add := func(col, val string) {
		if val != "" {
			args = append(args, val)
			conds = append(conds, fmt.Sprintf("%s = $%d", col, len(args)))
		}
	}
	add("tuple_user", filter.User)
	add("tuple_relation", filter.Relation)
	add("tuple_object", filter.Object)
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var out []domain.RelationTuple
	err := WithTx(ctx, p.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			"SELECT tuple_user, tuple_relation, tuple_object FROM authz_tuple "+where, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t domain.RelationTuple
			if err := rows.Scan(&t.User, &t.Relation, &t.Object); err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("pdp: leitura de tuplas falhou: %w", err)
	}
	return out, nil
}

// loadTenantGraph loads the tenant's tuples into an in-memory graph for a single
// decision — fresh every time (no cross-request cache). The organization_id
// predicate is explicit (Barreira 1): the graph never mixes tenants.
func (p *PostgresPDP) loadTenantGraph(ctx context.Context, org string) (*domain.MemoryGraph, error) {
	g := domain.NewMemoryGraph()
	err := WithTx(ctx, p.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT tuple_user, tuple_relation, tuple_object,
			       condition_name, not_before, expires_at
			FROM authz_tuple
			WHERE organization_id = $1`, org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var user, relation, object string
			var condName *string
			var notBefore, expiresAt *time.Time
			if err := rows.Scan(&user, &relation, &object, &condName, &notBefore, &expiresAt); err != nil {
				return err
			}
			if condName != nil && *condName == domain.ConditionValidWindow && notBefore != nil && expiresAt != nil {
				g.AddConditioned(object, relation, user, domain.ValidityWindow{NotBefore: *notBefore, ExpiresAt: *expiresAt})
			} else {
				g.Add(object, relation, user)
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

// objectsOfType returns the distinct objects of a given tenant-qualified type
// present in the store (candidates for a reverse query).
func (p *PostgresPDP) objectsOfType(ctx context.Context, org, typ string) ([]string, error) {
	var out []string
	err := WithTx(ctx, p.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT tuple_object FROM authz_tuple
			WHERE organization_id = $1 AND tuple_object LIKE $2`, org, typ+":%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var obj string
			if err := rows.Scan(&obj); err != nil {
				return err
			}
			out = append(out, obj)
		}
		return rows.Err()
	})
	return out, err
}

// applyToStore applies one tuple update to authz_tuple idempotently, carrying a
// conditioned tuple's window. Shared by PostgresPDP.Write.
func applyToStore(ctx context.Context, tx pgx.Tx, org string, u domain.TupleUpdate) error {
	switch u.Op {
	case domain.TupleWrite:
		var condName *string
		var notBefore, expiresAt *time.Time
		if u.Condition != nil {
			condName = &u.Condition.Name
			nb, exp := u.Condition.Window.NotBefore, u.Condition.Window.ExpiresAt
			notBefore, expiresAt = &nb, &exp
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO authz_tuple
				(organization_id, tuple_user, tuple_relation, tuple_object,
				 condition_name, not_before, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tuple_user, tuple_relation, tuple_object)
			DO UPDATE SET condition_name = EXCLUDED.condition_name,
			              not_before     = EXCLUDED.not_before,
			              expires_at     = EXCLUDED.expires_at`,
			org, u.Tuple.User, u.Tuple.Relation, u.Tuple.Object, condName, notBefore, expiresAt)
		return err
	case domain.TupleDelete:
		_, err := tx.Exec(ctx, `
			DELETE FROM authz_tuple
			WHERE tuple_user = $1 AND tuple_relation = $2 AND tuple_object = $3`,
			u.Tuple.User, u.Tuple.Relation, u.Tuple.Object)
		return err
	default:
		return fmt.Errorf("pdp: operação desconhecida %q", u.Op)
	}
}

var _ domain.PolicyDecisionPoint = (*PostgresPDP)(nil)
