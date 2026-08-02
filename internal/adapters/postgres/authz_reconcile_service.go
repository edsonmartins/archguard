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
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReconcileService drives the authorization-projection reconciler (M4 Fase F, T-031): it
// builds each tenant's expected tuple set from the source of truth (tenantExpectedTuples,
// RLS-scoped) and hands it to the AuthzReconciler, which removes stale (extra) tuples and
// reports missing ones. It is the safety net that heals drift and stale access left by any
// path that bypasses the per-mutation projection (e.g. the identity-level bulk cascade,
// T-030c).
type ReconcileService struct {
	pool       *pgxpool.Pool
	reconciler *AuthzReconciler
}

// NewReconcileService builds the service over the runtime pool.
func NewReconcileService(pool *pgxpool.Pool) *ReconcileService {
	return &ReconcileService{pool: pool, reconciler: NewAuthzReconciler()}
}

// ReconcileTenant reconciles one tenant's projection against its expected set.
func (s *ReconcileService) ReconcileTenant(ctx context.Context, orgID uuid.UUID) (domain.ReconcileReport, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return domain.ReconcileReport{}, err
	}
	var expected []domain.RelationTuple
	if err := NewTenantRepository(s.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		e, err := tenantExpectedTuples(ctx, ttx, orgID)
		expected = e
		return err
	}); err != nil {
		return domain.ReconcileReport{}, fmt.Errorf("reconcile: expected-set do tenant %s falhou: %w", orgID, err)
	}
	return s.reconciler.Reconcile(ctx, s.pool, orgID, expected)
}

// ReconcileAll reconciles every tenant, returning the aggregate counts of removed
// (stale) and missing-alerted tuples. A single tenant failure is returned; the caller
// (scheduler) logs and moves on next tick.
func (s *ReconcileService) ReconcileAll(ctx context.Context) (removed, missing int, err error) {
	orgs, err := listOrgIDs(ctx, s.pool)
	if err != nil {
		return 0, 0, err
	}
	for _, org := range orgs {
		report, rerr := s.ReconcileTenant(ctx, org)
		if rerr != nil {
			return removed, missing, rerr
		}
		removed += len(report.Removed)
		missing += len(report.MissingAlerted)
	}
	return removed, missing, nil
}

// listOrgIDs returns the uuid ids of all organizations (pacote 002 gave each a stable id).
func listOrgIDs(ctx context.Context, pool *pgxpool.Pool) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `SELECT id::text FROM organization WHERE id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("reconcile: listagem de organizações falhou: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(s)
		if perr != nil {
			continue // orgs legadas sem uuid válido são ignoradas
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
