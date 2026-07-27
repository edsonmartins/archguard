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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantGrantLister lists a tenant's active privileged grants, tenant-scoped
// through WithTenantTx (RLS SET LOCAL + explicit predicate), so it only ever
// returns the requested tenant's grants.
type TenantGrantLister struct {
	pool *pgxpool.Pool
}

// NewTenantGrantLister builds the lister over the runtime pool.
func NewTenantGrantLister(pool *pgxpool.Pool) *TenantGrantLister {
	return &TenantGrantLister{pool: pool}
}

// ListActive returns the active privileged grants in the given organization. The
// caller must pass the session's active tenant, never an arbitrary org.
func (r *TenantGrantLister) ListActive(ctx context.Context, orgID uuid.UUID) ([]domain.PrivilegedGrant, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.PrivilegedGrant
	err = NewTenantRepository(r.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		gs, lerr := NewPrivilegedGrantStore(ttx).ListActive(ctx)
		if lerr != nil {
			return lerr
		}
		out = gs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListAwaitingApproval returns the tenant's break-glass grants awaiting peer approval —
// the approval queue (T-008). The caller must pass the session's active tenant.
func (r *TenantGrantLister) ListAwaitingApproval(ctx context.Context, orgID uuid.UUID) ([]domain.PrivilegedGrant, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.PrivilegedGrant
	err = NewTenantRepository(r.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		gs, lerr := NewPrivilegedGrantStore(ttx).ListAwaitingApproval(ctx)
		if lerr != nil {
			return lerr
		}
		out = gs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
