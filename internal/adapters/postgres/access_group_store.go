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

// AccessGroupStore persists the tenant's access-group catalog on a TenantTx. A group is
// metadata only (no projection): the `member`/operator tuples come from group_membership
// and from assignments.
type AccessGroupStore struct {
	ttx *TenantTx
}

// NewAccessGroupStore builds the store on an open tenant transaction.
func NewAccessGroupStore(ttx *TenantTx) *AccessGroupStore {
	return &AccessGroupStore{ttx: ttx}
}

// Create inserts an access group. Refuses a cross-tenant row.
func (s *AccessGroupStore) Create(ctx context.Context, g domain.AccessGroup) error {
	if g.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, g.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `INSERT INTO access_group (id, organization_id, name, display_name) VALUES ($1, $2, $3, $4)`
	if _, err := s.ttx.tx.Exec(ctx, q, g.ID.String(), g.OrganizationID.String(), g.Name, g.DisplayName); err != nil {
		return fmt.Errorf("postgres: criação de access_group falhou: %w", err)
	}
	return nil
}

// List returns the tenant's access groups, by name.
func (s *AccessGroupStore) List(ctx context.Context) ([]domain.AccessGroup, error) {
	const q = `SELECT id, organization_id, name, display_name FROM access_group ORDER BY name`
	rows, err := s.ttx.tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de access_group falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.AccessGroup
	for rows.Next() {
		var g domain.AccessGroup
		var idStr, orgStr string
		if err := rows.Scan(&idStr, &orgStr, &g.Name, &g.DisplayName); err != nil {
			return nil, fmt.Errorf("postgres: scan de access_group falhou: %w", err)
		}
		g.ID = uuid.MustParse(idStr)
		g.OrganizationID = uuid.MustParse(orgStr)
		out = append(out, g)
	}
	return out, rows.Err()
}

// AccessGroupCatalog is the pool-level facade the HTTP handler uses (tenant from the
// session, INV-1).
type AccessGroupCatalog struct {
	pool *pgxpool.Pool
}

// NewAccessGroupCatalog builds the catalog over the runtime pool.
func NewAccessGroupCatalog(pool *pgxpool.Pool) *AccessGroupCatalog {
	return &AccessGroupCatalog{pool: pool}
}

// ListInTenant returns the access groups of the given tenant.
func (c *AccessGroupCatalog) ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.AccessGroup, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.AccessGroup
	err = NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		g, e := NewAccessGroupStore(ttx).List(ctx)
		out = g
		return e
	})
	return out, err
}

// CreateInTenant creates an access group in the given tenant.
func (c *AccessGroupCatalog) CreateInTenant(ctx context.Context, orgID uuid.UUID, g domain.AccessGroup) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	return NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewAccessGroupStore(ttx).Create(ctx, g)
	})
}
