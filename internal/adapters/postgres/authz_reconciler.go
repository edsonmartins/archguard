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

// AuthzReconciler reconciles the PDP projection (authz_tuple) of ONE tenant with
// the tuple set expected from the source of truth, applying the asymmetric policy
// (RFC-0004 §6): extra tuples (over-permissioning) are removed automatically;
// missing tuples (would amplify access) are reported for human review, never
// added automatically. It reconciles per tenant so the blast radius is bounded and
// the caller must supply the tenant's COMPLETE expected set (a partial set would
// classify real tuples as extra — see domain.ReconcileDiff).
type AuthzReconciler struct{}

// NewAuthzReconciler builds the reconciler.
func NewAuthzReconciler() *AuthzReconciler { return &AuthzReconciler{} }

// Reconcile compares the store tuples of orgID with expected and applies the
// asymmetric policy. Extra tuples are DELETED in one transaction; missing tuples
// are returned in the report for alerting (T-019) and NOT applied. expected must
// be the tenant's authoritative, complete derived set.
func (r *AuthzReconciler) Reconcile(ctx context.Context, db Beginner, orgID uuid.UUID, expected []domain.RelationTuple) (domain.ReconcileReport, error) {
	actual, err := r.loadTenantTuples(ctx, db, orgID)
	if err != nil {
		return domain.ReconcileReport{}, err
	}

	extra, missing := domain.ReconcileDiff(expected, actual)

	if len(extra) > 0 {
		if err := WithTx(ctx, db, func(tx pgx.Tx) error {
			for _, tup := range extra {
				if _, err := tx.Exec(ctx, `
					DELETE FROM authz_tuple
					WHERE organization_id = $1
					  AND tuple_user = $2 AND tuple_relation = $3 AND tuple_object = $4`,
					orgID.String(), tup.User, tup.Relation, tup.Object); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return domain.ReconcileReport{}, fmt.Errorf("authz_reconciler: remoção de tuplas extras falhou: %w", err)
		}
	}

	return domain.ReconcileReport{Removed: extra, MissingAlerted: missing}, nil
}

// loadTenantTuples reads all store tuples of one tenant. The organization_id
// predicate is explicit (Barreira 1): the reconciler never scans across tenants.
func (r *AuthzReconciler) loadTenantTuples(ctx context.Context, db Beginner, orgID uuid.UUID) ([]domain.RelationTuple, error) {
	var out []domain.RelationTuple
	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT tuple_user, tuple_relation, tuple_object
			FROM authz_tuple
			WHERE organization_id = $1`, orgID.String())
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
		return nil, fmt.Errorf("authz_reconciler: leitura das tuplas do tenant falhou: %w", err)
	}
	return out, nil
}
