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

// AssetAccessStore persists granular access assignments (subject→operator/auditor→
// asset/asset_group) on a TenantTx, and ENQUEUES the derived tuple to the AuthzOutbox
// in the same transaction (M4 T-029, RFC-0004 §4). The publisher drains it to the
// projection, from which the PDP resolves operator (direct) and inherited access.
type AssetAccessStore struct {
	ttx *TenantTx
}

// NewAssetAccessStore builds the store on an open tenant transaction.
func NewAssetAccessStore(ttx *TenantTx) *AssetAccessStore {
	return &AssetAccessStore{ttx: ttx}
}

// Create inserts an assignment and enqueues its projection. Refuses a row whose tenant
// differs from the store's.
func (s *AssetAccessStore) Create(ctx context.Context, a domain.AssetAccessAssignment) error {
	if a.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, a.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `INSERT INTO asset_access_assignment
	           (id, organization_id, subject_type, subject_id, relation, object_type, object_id)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		a.ID.String(), a.OrganizationID.String(), string(a.SubjectType), a.SubjectID.String(), a.Relation,
		string(a.ObjectType), a.ObjectID.String()); err != nil {
		return fmt.Errorf("postgres: criação de asset_access_assignment falhou: %w", err)
	}
	update, err := a.Tuple(true)
	if err != nil {
		return err
	}
	return NewAuthzOutbox(s.ttx.Tx()).Enqueue(ctx, []domain.TupleUpdate{update})
}

// List returns the tenant's access assignments, newest first.
func (s *AssetAccessStore) List(ctx context.Context) ([]domain.AssetAccessAssignment, error) {
	const q = `SELECT id, organization_id, subject_type, subject_id, relation, object_type, object_id
	           FROM asset_access_assignment ORDER BY created_at DESC`
	rows, err := s.ttx.tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de asset_access falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.AssetAccessAssignment
	for rows.Next() {
		var a domain.AssetAccessAssignment
		var idStr, orgStr, subjTypeStr, subjStr, objTypeStr, objStr string
		if err := rows.Scan(&idStr, &orgStr, &subjTypeStr, &subjStr, &a.Relation, &objTypeStr, &objStr); err != nil {
			return nil, fmt.Errorf("postgres: scan de asset_access falhou: %w", err)
		}
		a.ID = uuid.MustParse(idStr)
		a.OrganizationID = uuid.MustParse(orgStr)
		a.SubjectType = domain.ObjectType(subjTypeStr)
		a.SubjectID = uuid.MustParse(subjStr)
		a.ObjectType = domain.ObjectType(objTypeStr)
		a.ObjectID = uuid.MustParse(objStr)
		out = append(out, a)
	}
	return out, rows.Err()
}

// AssetAccessCatalog is the pool-level facade the HTTP handler uses: each call opens a
// TenantTx confined to the session's tenant (the org comes from the session, INV-1).
type AssetAccessCatalog struct {
	pool *pgxpool.Pool
}

// NewAssetAccessCatalog builds the catalog over the runtime pool.
func NewAssetAccessCatalog(pool *pgxpool.Pool) *AssetAccessCatalog {
	return &AssetAccessCatalog{pool: pool}
}

// ListInTenant returns the assignments of the given tenant.
func (c *AssetAccessCatalog) ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.AssetAccessAssignment, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.AssetAccessAssignment
	err = NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		a, e := NewAssetAccessStore(ttx).List(ctx)
		out = a
		return e
	})
	return out, err
}

// CreateInTenant creates an assignment in the given tenant (mutation + projection atomic).
func (c *AssetAccessCatalog) CreateInTenant(ctx context.Context, orgID uuid.UUID, a domain.AssetAccessAssignment) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	return NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewAssetAccessStore(ttx).Create(ctx, a)
	})
}
