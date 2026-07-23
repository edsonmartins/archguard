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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Errors of the privileged-grant store.
var (
	ErrGrantNotFound    = errors.New("postgres: concessão privilegiada não encontrada")
	ErrCrossTenantGrant = errors.New("postgres: concessão de outra organização recusada")
)

// PrivilegedGrantStore is the tenant-scoped store for privileged_grant (pacote
// 004). Built on a TenantTx, it carries the explicit organization_id predicate
// (Barreira 1) and the SET LOCAL tenant setting the RLS policy reads (Barreira 2).
type PrivilegedGrantStore struct {
	ttx *TenantTx
}

// NewPrivilegedGrantStore builds the store on an open tenant transaction.
func NewPrivilegedGrantStore(ttx *TenantTx) *PrivilegedGrantStore {
	return &PrivilegedGrantStore{ttx: ttx}
}

// Create persists a grant. It refuses a grant of another organization.
func (s *PrivilegedGrantStore) Create(ctx context.Context, g domain.PrivilegedGrant) error {
	if g.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantGrant, g.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `
		INSERT INTO privileged_grant
			(id, organization_id, subject_membership_id, target_type, target_id, target_scope,
			 origin, status, required_approvals, not_before, expires_at, justification, incident_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := s.ttx.tx.Exec(ctx, q,
		g.ID.String(), g.OrganizationID.String(), g.SubjectMembershipID.String(),
		g.Target.Type, g.Target.ID, g.Target.Scope, string(g.Origin), string(g.Status),
		g.RequiredApprovals, g.NotBefore, g.ExpiresAt, g.Justification, g.IncidentRef)
	if err != nil {
		return fmt.Errorf("postgres: criação de concessão falhou: %w", err)
	}
	return nil
}

// Get loads a grant and its approvals within the tenant scope.
func (s *PrivilegedGrantStore) Get(ctx context.Context, id uuid.UUID) (domain.PrivilegedGrant, error) {
	const q = `
		SELECT id::text, organization_id::text, subject_membership_id::text,
		       target_type, target_id, target_scope, origin, status, required_approvals,
		       not_before, expires_at, justification, incident_ref
		FROM privileged_grant
		WHERE id = $1 AND organization_id = $2`
	g, err := scanGrant(s.ttx.tx.QueryRow(ctx, q, id.String(), s.ttx.scope.OrganizationID().String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PrivilegedGrant{}, ErrGrantNotFound
	}
	if err != nil {
		return domain.PrivilegedGrant{}, err
	}
	approvals, err := s.loadApprovals(ctx, id)
	if err != nil {
		return domain.PrivilegedGrant{}, err
	}
	g.Approvals = approvals
	return g, nil
}

func (s *PrivilegedGrantStore) loadApprovals(ctx context.Context, grantID uuid.UUID) ([]domain.GrantApproval, error) {
	rows, err := s.ttx.tx.Query(ctx,
		`SELECT approver_membership_id::text FROM grant_approval
		 WHERE grant_id = $1 AND organization_id = $2 ORDER BY created_at`,
		grantID.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return nil, fmt.Errorf("postgres: leitura de aprovações de concessão falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.GrantApproval
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("postgres: leitura de aprovação falhou: %w", err)
		}
		id, err := uuid.Parse(text)
		if err != nil {
			return nil, fmt.Errorf("postgres: aprovador inválido: %w", err)
		}
		out = append(out, domain.GrantApproval{ApproverMembershipID: id})
	}
	return out, rows.Err()
}

// SaveDecision persists the grant's status and upserts its approvals (composite
// PK enforces distinct approvers) within the caller's transaction.
func (s *PrivilegedGrantStore) SaveDecision(ctx context.Context, g domain.PrivilegedGrant) error {
	if g.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantGrant, g.OrganizationID, s.ttx.scope.OrganizationID())
	}
	for _, a := range g.Approvals {
		if _, err := s.ttx.tx.Exec(ctx,
			`INSERT INTO grant_approval (grant_id, approver_membership_id, organization_id)
			 VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			g.ID.String(), a.ApproverMembershipID.String(), g.OrganizationID.String()); err != nil {
			return fmt.Errorf("postgres: gravação de aprovação de concessão falhou: %w", err)
		}
	}
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE privileged_grant SET status = $3, updated_at = now()
		 WHERE id = $1 AND organization_id = $2`,
		g.ID.String(), g.OrganizationID.String(), string(g.Status))
	if err != nil {
		return fmt.Errorf("postgres: atualização de concessão falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrGrantNotFound
	}
	return nil
}

// ListActiveExpired returns the tenant's ACTIVE grants whose window has already
// passed at now — the work list of the expiry job (T-012).
func (s *PrivilegedGrantStore) ListActiveExpired(ctx context.Context, now time.Time) ([]domain.PrivilegedGrant, error) {
	const q = `
		SELECT id::text, organization_id::text, subject_membership_id::text,
		       target_type, target_id, target_scope, origin, status, required_approvals,
		       not_before, expires_at, justification, incident_ref
		FROM privileged_grant
		WHERE organization_id = $1 AND status = 'active' AND expires_at <= $2
		ORDER BY expires_at`
	rows, err := s.ttx.tx.Query(ctx, q, s.ttx.scope.OrganizationID().String(), now)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de concessões expiradas falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.PrivilegedGrant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// scanGrant reads one privileged_grant row (see the shared column order).
func scanGrant(row pgx.Row) (domain.PrivilegedGrant, error) {
	var g domain.PrivilegedGrant
	var idText, orgText, subText, origin, status string
	if err := row.Scan(&idText, &orgText, &subText,
		&g.Target.Type, &g.Target.ID, &g.Target.Scope, &origin, &status, &g.RequiredApprovals,
		&g.NotBefore, &g.ExpiresAt, &g.Justification, &g.IncidentRef); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PrivilegedGrant{}, err
		}
		return domain.PrivilegedGrant{}, fmt.Errorf("postgres: leitura de concessão falhou: %w", err)
	}
	var err error
	if g.ID, err = uuid.Parse(idText); err != nil {
		return domain.PrivilegedGrant{}, fmt.Errorf("postgres: id de concessão inválido: %w", err)
	}
	if g.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.PrivilegedGrant{}, fmt.Errorf("postgres: organização inválida: %w", err)
	}
	if g.SubjectMembershipID, err = uuid.Parse(subText); err != nil {
		return domain.PrivilegedGrant{}, fmt.Errorf("postgres: subject inválido: %w", err)
	}
	g.Origin = domain.GrantOrigin(origin)
	g.Status = domain.GrantStatus(status)
	return g, nil
}
