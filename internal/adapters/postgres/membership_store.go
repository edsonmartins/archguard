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

// MembershipStore reads memberships. Membership access is inherently two-sided:
// "the members of THIS organization" is tenant-scoped (Barreira 1), while "ALL of
// MY memberships across organizations" is a legitimate CROSS-TENANT read — the
// canonical GlobalRepository use (RFC-0002 §4). ListByIdentity is the latter and
// must run through GlobalRepository, so the cross-tenant read is authorized and
// audited.
type MembershipStore struct {
	q Querier
}

// NewMembershipStore builds a store over any Querier — pass the pgx.Tx from
// GlobalRepository.WithGlobalTx for the cross-tenant read.
func NewMembershipStore(q Querier) *MembershipStore {
	return &MembershipStore{q: q}
}

// ListByIdentity returns every membership of an identity, ACROSS all
// organizations. Under RLS (T-010) this only returns rows when the caller is in
// global-read mode — which is exactly what GlobalRepository establishes.
func (s *MembershipStore) ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]domain.Membership, error) {
	const q = `
		SELECT id::text, identity_id::text, organization_id::text, status, created_at, updated_at
		FROM membership
		WHERE identity_id = $1
		ORDER BY created_at`
	rows, err := s.q.Query(ctx, q, identityID.String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de memberships falhou: %w", err)
	}
	defer rows.Close()

	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		var idText, idnText, orgText, status string
		if err := rows.Scan(&idText, &idnText, &orgText, &status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: leitura de membership falhou: %w", err)
		}
		if m.ID, err = uuid.Parse(idText); err != nil {
			return nil, fmt.Errorf("postgres: id de membership inválido %q: %w", idText, err)
		}
		if m.IdentityID, err = uuid.Parse(idnText); err != nil {
			return nil, fmt.Errorf("postgres: identity_id inválido %q: %w", idnText, err)
		}
		if m.OrganizationID, err = uuid.Parse(orgText); err != nil {
			return nil, fmt.Errorf("postgres: organization_id inválido %q: %w", orgText, err)
		}
		m.Status = domain.MembershipStatus(status)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de memberships falhou: %w", err)
	}
	return out, nil
}
