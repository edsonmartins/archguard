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
)

// RoleAssignmentStore persists role↔membership bindings (RFC-0002 §2.4, R2).
// role_assignment is a tenant-scoped domain table; once RLS is enabled (T-010)
// this store operates under the tenant session variable. It carries no tenant
// context of its own yet — the tenant-scoped repository wrapper is T-008.
type RoleAssignmentStore struct {
	db Querier
}

// NewRoleAssignmentStore builds a store over any Querier (pool or transaction).
func NewRoleAssignmentStore(db Querier) *RoleAssignmentStore {
	return &RoleAssignmentStore{db: db}
}

// Create inserts a role assignment. A duplicate (role_id, membership_id) surfaces
// as the underlying unique-constraint error.
func (s *RoleAssignmentStore) Create(ctx context.Context, ra domain.RoleAssignment) error {
	const q = `
		INSERT INTO role_assignment (id, organization_id, role_id, membership_id)
		VALUES ($1, $2, $3, $4)`
	_, err := s.db.Exec(ctx, q,
		ra.ID.String(), ra.OrganizationID.String(), ra.RoleID.String(), ra.MembershipID.String())
	if err != nil {
		return fmt.Errorf("postgres: criação de role_assignment falhou: %w", err)
	}
	return nil
}

// ListByMembership returns the role assignments of one membership.
func (s *RoleAssignmentStore) ListByMembership(ctx context.Context, membershipID uuid.UUID) ([]domain.RoleAssignment, error) {
	const q = `
		SELECT id::text, organization_id::text, role_id::text, membership_id::text
		FROM role_assignment
		WHERE membership_id = $1
		ORDER BY created_at`
	rows, err := s.db.Query(ctx, q, membershipID.String())
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
