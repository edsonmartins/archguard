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
	"errors"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ErrCrossTenantWrite is returned when a tenant-scoped store is asked to persist
// a row belonging to a different tenant than its scope — Barreira 1 applied to
// writes (RFC-0002 §4): a repository bound to tenant A cannot create data in B.
var ErrCrossTenantWrite = errors.New("postgres: escrita cross-tenant recusada (Barreira 1)")

// RoleAssignmentStore is the tenant-scoped store for role↔membership bindings
// (RFC-0002 §2.4, R2). It is built on a TenantTx, so every operation is confined
// to that tenant two ways: an explicit organization_id predicate/guard (Barreira
// 1, effective even with RLS off) and the SET LOCAL setting the TenantTx already
// applied (Barreira 2 / RLS, T-010).
type RoleAssignmentStore struct {
	ttx *TenantTx
}

// NewRoleAssignmentStore builds the store on an open tenant transaction. There is
// no constructor without a tenant.
func NewRoleAssignmentStore(ttx *TenantTx) *RoleAssignmentStore {
	return &RoleAssignmentStore{ttx: ttx}
}

// Create inserts a role assignment. It refuses a row whose organization differs
// from the store's tenant (ErrCrossTenantWrite). A duplicate (role_id,
// membership_id) surfaces as the underlying unique-constraint error.
func (s *RoleAssignmentStore) Create(ctx context.Context, ra domain.RoleAssignment) error {
	scope := s.ttx.scope.OrganizationID()
	if ra.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, ra.OrganizationID, scope)
	}
	const q = `
		INSERT INTO role_assignment (id, organization_id, role_id, membership_id)
		VALUES ($1, $2, $3, $4)`
	_, err := s.ttx.tx.Exec(ctx, q,
		ra.ID.String(), ra.OrganizationID.String(), ra.RoleID.String(), ra.MembershipID.String())
	if err != nil {
		return fmt.Errorf("postgres: criação de role_assignment falhou: %w", err)
	}
	return nil
}

// ListByMembership returns the role assignments of one membership WITHIN the
// store's tenant. The explicit organization_id predicate is Barreira 1: even if
// RLS were off, a store scoped to tenant A never returns tenant B's rows.
func (s *RoleAssignmentStore) ListByMembership(ctx context.Context, membershipID uuid.UUID) ([]domain.RoleAssignment, error) {
	const q = `
		SELECT id::text, organization_id::text, role_id::text, membership_id::text
		FROM role_assignment
		WHERE membership_id = $1 AND organization_id = $2
		ORDER BY created_at`
	rows, err := s.ttx.tx.Query(ctx, q, membershipID.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de role_assignment falhou: %w", err)
	}
	defer rows.Close()

	var out []domain.RoleAssignment
	for rows.Next() {
		var idText, orgText, roleText, memText string
		if err := rows.Scan(&idText, &orgText, &roleText, &memText); err != nil {
			return nil, fmt.Errorf("postgres: leitura de role_assignment falhou: %w", err)
		}
		ra, err := parseRoleAssignment(idText, orgText, roleText, memText)
		if err != nil {
			return nil, err
		}
		out = append(out, ra)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de role_assignment falhou: %w", err)
	}
	return out, nil
}

func parseRoleAssignment(idText, orgText, roleText, memText string) (domain.RoleAssignment, error) {
	ids := map[string]string{"id": idText, "organization_id": orgText, "role_id": roleText, "membership_id": memText}
	parsed := map[string]uuid.UUID{}
	for k, v := range ids {
		u, err := uuid.Parse(v)
		if err != nil {
			return domain.RoleAssignment{}, fmt.Errorf("postgres: %s inválido %q: %w", k, v, err)
		}
		parsed[k] = u
	}
	return domain.RoleAssignment{
		ID:             parsed["id"],
		OrganizationID: parsed["organization_id"],
		RoleID:         parsed["role_id"],
		MembershipID:   parsed["membership_id"],
	}, nil
}
