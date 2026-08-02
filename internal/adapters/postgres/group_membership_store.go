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

// GroupMembershipStore persists membership↔access-group bindings on a TenantTx and
// ENQUEUES the derived `member` tuple to the AuthzOutbox in the same transaction (M4
// T-029 D1, RFC-0004 §4).
type GroupMembershipStore struct {
	ttx *TenantTx
}

// NewGroupMembershipStore builds the store on an open tenant transaction.
func NewGroupMembershipStore(ttx *TenantTx) *GroupMembershipStore {
	return &GroupMembershipStore{ttx: ttx}
}

// Create inserts a binding and enqueues its `member` write. Refuses a cross-tenant row.
func (s *GroupMembershipStore) Create(ctx context.Context, g domain.GroupMembership) error {
	if g.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, g.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `INSERT INTO group_membership (id, organization_id, group_id, membership_id)
	           VALUES ($1, $2, $3, $4)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		g.ID.String(), g.OrganizationID.String(), g.GroupID.String(), g.MembershipID.String()); err != nil {
		return fmt.Errorf("postgres: criação de group_membership falhou: %w", err)
	}
	return NewAuthzOutbox(s.ttx.Tx()).Enqueue(ctx, []domain.TupleUpdate{g.Tuple(true)})
}

// Delete removes a binding by (group, membership) and enqueues the `member` delete.
func (s *GroupMembershipStore) Delete(ctx context.Context, g domain.GroupMembership) error {
	if g.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, g.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `DELETE FROM group_membership
	           WHERE organization_id = $1 AND group_id = $2 AND membership_id = $3`
	if _, err := s.ttx.tx.Exec(ctx, q, g.OrganizationID.String(), g.GroupID.String(), g.MembershipID.String()); err != nil {
		return fmt.Errorf("postgres: remoção de group_membership falhou: %w", err)
	}
	return NewAuthzOutbox(s.ttx.Tx()).Enqueue(ctx, []domain.TupleUpdate{g.Tuple(false)})
}

// List returns the tenant's group bindings, newest first.
func (s *GroupMembershipStore) List(ctx context.Context) ([]domain.GroupMembership, error) {
	const q = `SELECT id, organization_id, group_id, membership_id
	           FROM group_membership ORDER BY created_at DESC`
	rows, err := s.ttx.tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de group_membership falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.GroupMembership
	for rows.Next() {
		var g domain.GroupMembership
		var idStr, orgStr, grpStr, memStr string
		if err := rows.Scan(&idStr, &orgStr, &grpStr, &memStr); err != nil {
			return nil, fmt.Errorf("postgres: scan de group_membership falhou: %w", err)
		}
		g.ID = uuid.MustParse(idStr)
		g.OrganizationID = uuid.MustParse(orgStr)
		g.GroupID = uuid.MustParse(grpStr)
		g.MembershipID = uuid.MustParse(memStr)
		out = append(out, g)
	}
	return out, rows.Err()
}

// GroupMembershipCatalog is the pool-level facade the HTTP handler uses (tenant from the
// session, INV-1).
type GroupMembershipCatalog struct {
	pool *pgxpool.Pool
}

// NewGroupMembershipCatalog builds the catalog over the runtime pool.
func NewGroupMembershipCatalog(pool *pgxpool.Pool) *GroupMembershipCatalog {
	return &GroupMembershipCatalog{pool: pool}
}

// ListInTenant returns the bindings of the given tenant.
func (c *GroupMembershipCatalog) ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.GroupMembership, error) {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.GroupMembership
	err = NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		g, e := NewGroupMembershipStore(ttx).List(ctx)
		out = g
		return e
	})
	return out, err
}

// CreateInTenant creates a binding in the given tenant (mutation + projection atomic).
func (c *GroupMembershipCatalog) CreateInTenant(ctx context.Context, orgID uuid.UUID, g domain.GroupMembership) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	return NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewGroupMembershipStore(ttx).Create(ctx, g)
	})
}
