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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Errors of the tenant-scoped membership store.
var (
	// ErrMembershipNotFound is returned when the membership does not exist WITHIN
	// the store's tenant — one of another organization is indistinguishable from
	// a missing one, on purpose.
	ErrMembershipNotFound = errors.New("postgres: membership não encontrado")
	// ErrAlreadyMember is returned when the identity already holds a
	// (non-revoked) membership in the organization — R3: one membership per
	// (identity, organization) pair. Inviting an existing member is a no-op
	// refused, never a duplicate.
	ErrAlreadyMember = errors.New("postgres: identidade já possui membership nesta organização")
	// ErrPreviouslyRevoked is returned when the pair has a REVOKED membership.
	// R3 as written keeps the pair unique and the revoked row stays for the
	// audit trail, so readmission is impossible today — supporting it requires
	// an RFC-0002 amendment (partial unique index excluding revoked). Decisão do
	// arquiteto (2026-07-21): R3 estrita; a tensão fica registrada.
	ErrPreviouslyRevoked = errors.New("postgres: par identidade/organização tem membership revogado — readmissão exige emenda da R3 (RFC-0002)")
	// ErrNotInvited is returned when persisting an activation for a membership
	// that is not in the invited state (already active, suspended, or revoked).
	ErrNotInvited = errors.New("postgres: membership não está pendente de aceite")
)

// pgUniqueViolation is the SQLSTATE of a unique-constraint violation.
const pgUniqueViolation = "23505"

// TenantMembershipStore is the tenant-scoped store for memberships: "the
// members of THIS organization" (the cross-tenant "all MY memberships" read
// lives in MembershipStore, under the GlobalRepository). Built on a TenantTx,
// so every operation carries the explicit organization_id predicate (Barreira
// 1) and the SET LOCAL tenant setting the RLS policies read (Barreira 2).
type TenantMembershipStore struct {
	ttx *TenantTx
}

// NewTenantMembershipStore builds the store on an open tenant transaction.
// There is no constructor without a tenant.
func NewTenantMembershipStore(ttx *TenantTx) *TenantMembershipStore {
	return &TenantMembershipStore{ttx: ttx}
}

// Create inserts a membership in the store's tenant. It refuses a row of
// another organization (ErrCrossTenantWrite) and classifies the R3 collision:
// an existing non-revoked membership yields ErrAlreadyMember, a revoked one
// ErrPreviouslyRevoked (readmission needs an RFC amendment — see the error).
func (s *TenantMembershipStore) Create(ctx context.Context, m domain.Membership) error {
	scope := s.ttx.scope.OrganizationID()
	if m.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, m.OrganizationID, scope)
	}

	// Classify an existing pair first so the caller gets a meaningful refusal.
	// A concurrent insert between this check and ours still surfaces as the
	// unique violation below — never a duplicate row.
	var status string
	err := s.ttx.tx.QueryRow(ctx,
		"SELECT status FROM membership WHERE identity_id = $1 AND organization_id = $2",
		m.IdentityID.String(), scope.String()).Scan(&status)
	switch {
	case err == nil:
		if status == string(domain.MembershipRevoked) {
			return ErrPreviouslyRevoked
		}
		return ErrAlreadyMember
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("postgres: verificação de membership existente falhou: %w", err)
	}

	const q = `
		INSERT INTO membership (id, identity_id, organization_id, status, attributes_enc, invited_by)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = s.ttx.tx.Exec(ctx, q,
		m.ID.String(), m.IdentityID.String(), m.OrganizationID.String(),
		string(m.Status), m.AttributesEnc, uuidTextOrNil(m.InvitedBy))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return ErrAlreadyMember
		}
		return fmt.Errorf("postgres: criação de membership falhou: %w", err)
	}
	return nil
}

// Get returns one membership of the store's tenant. A membership of another
// organization yields ErrMembershipNotFound — the predicate makes it
// unreachable.
func (s *TenantMembershipStore) Get(ctx context.Context, membershipID uuid.UUID) (domain.Membership, error) {
	const q = `
		SELECT id::text, identity_id::text, organization_id::text, status,
		       invited_by::text, activated_at, created_at, updated_at
		FROM membership
		WHERE id = $1 AND organization_id = $2`
	row := s.ttx.tx.QueryRow(ctx, q, membershipID.String(), s.ttx.scope.OrganizationID().String())

	var m domain.Membership
	var idText, idnText, orgText, status string
	var invitedByText *string
	err := row.Scan(&idText, &idnText, &orgText, &status,
		&invitedByText, &m.ActivatedAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, ErrMembershipNotFound
	}
	if err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: leitura de membership falhou: %w", err)
	}
	if m.ID, err = uuid.Parse(idText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: id de membership inválido %q: %w", idText, err)
	}
	if m.IdentityID, err = uuid.Parse(idnText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: identity_id inválido %q: %w", idnText, err)
	}
	if m.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: organization_id inválido %q: %w", orgText, err)
	}
	if m.InvitedBy, err = parseOptionalUUID("invited_by", invitedByText); err != nil {
		return domain.Membership{}, err
	}
	m.Status = domain.MembershipStatus(status)
	return m, nil
}

// ListInTenant returns every membership in the store's tenant (the active org),
// ordered by creation. Explicitly predicated on the tenant (Barreira 1) on top of
// the WithTenantTx RLS scope (Barreira 2) — INV-5.
func (s *TenantMembershipStore) ListInTenant(ctx context.Context) ([]domain.Membership, error) {
	const q = `
		SELECT id::text, identity_id::text, organization_id::text, status,
		       invited_by::text, activated_at, created_at, updated_at
		FROM membership
		WHERE organization_id = $1
		ORDER BY created_at`
	rows, err := s.ttx.tx.Query(ctx, q, s.ttx.scope.OrganizationID().String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de memberships falhou: %w", err)
	}
	defer rows.Close()

	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		var idText, idnText, orgText, status string
		var invitedByText *string
		if err := rows.Scan(&idText, &idnText, &orgText, &status,
			&invitedByText, &m.ActivatedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
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
		if m.InvitedBy, err = parseOptionalUUID("invited_by", invitedByText); err != nil {
			return nil, err
		}
		m.Status = domain.MembershipStatus(status)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de memberships falhou: %w", err)
	}
	return out, nil
}

// SaveRevocation persists a membership revocation (domain.Membership.Revoke
// already ran — terminal, R4). Idempotent: re-revoking keeps the original
// revoked_at. The row stays for the audit trail; the caller cascades to the
// tenant's sessions (MembershipRevoker).
func (s *TenantMembershipStore) SaveRevocation(ctx context.Context, m domain.Membership) error {
	scope := s.ttx.scope.OrganizationID()
	if m.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, m.OrganizationID, scope)
	}
	const q = `
		UPDATE membership
		SET status = $3, revoked_at = COALESCE(revoked_at, now()), updated_at = now()
		WHERE id = $1 AND organization_id = $2`
	tag, err := s.ttx.tx.Exec(ctx, q,
		m.ID.String(), scope.String(), string(domain.MembershipRevoked))
	if err != nil {
		return fmt.Errorf("postgres: gravação de revogação de membership falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

// FindByIdentity returns the identity's membership in the store's tenant, or
// ErrMembershipNotFound. There is at most one (identity, organization) pair.
func (s *TenantMembershipStore) FindByIdentity(ctx context.Context, identityID uuid.UUID) (domain.Membership, error) {
	const q = `
		SELECT id::text, identity_id::text, organization_id::text, status,
		       invited_by::text, activated_at, created_at, updated_at
		FROM membership
		WHERE identity_id = $1 AND organization_id = $2`
	row := s.ttx.tx.QueryRow(ctx, q, identityID.String(), s.ttx.scope.OrganizationID().String())

	var m domain.Membership
	var idText, idnText, orgText, status string
	var invitedByText *string
	err := row.Scan(&idText, &idnText, &orgText, &status,
		&invitedByText, &m.ActivatedAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, ErrMembershipNotFound
	}
	if err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: leitura de membership por identidade falhou: %w", err)
	}
	if m.ID, err = uuid.Parse(idText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: id de membership inválido %q: %w", idText, err)
	}
	if m.IdentityID, err = uuid.Parse(idnText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: identity_id inválido %q: %w", idnText, err)
	}
	if m.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.Membership{}, fmt.Errorf("postgres: organization_id inválido %q: %w", orgText, err)
	}
	if m.InvitedBy, err = parseOptionalUUID("invited_by", invitedByText); err != nil {
		return domain.Membership{}, err
	}
	m.Status = domain.MembershipStatus(status)
	return m, nil
}

// SaveReactivation persists a membership resume (domain.Membership.Resume already
// ran, suspended → active). The UPDATE only matches a SUSPENDED row, so it is
// idempotent and never resurrects a revoked membership.
func (s *TenantMembershipStore) SaveReactivation(ctx context.Context, m domain.Membership) error {
	scope := s.ttx.scope.OrganizationID()
	if m.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, m.OrganizationID, scope)
	}
	const q = `
		UPDATE membership
		SET status = $3, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND status = $4`
	tag, err := s.ttx.tx.Exec(ctx, q,
		m.ID.String(), scope.String(),
		string(domain.MembershipActive), string(domain.MembershipSuspended))
	if err != nil {
		return fmt.Errorf("postgres: reativação de membership falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

// SaveSuspension persists a membership suspension (domain.Membership.Suspend
// already ran). The UPDATE only matches an ACTIVE row, so it is idempotent and
// safe: a revoked membership stays revoked and a concurrently-changed one is not
// clobbered. The row stays for the audit trail (never deleted); the caller ends
// the tenant's sessions. Zero rows affected yields ErrMembershipNotFound (the
// membership was not active to suspend).
func (s *TenantMembershipStore) SaveSuspension(ctx context.Context, m domain.Membership) error {
	scope := s.ttx.scope.OrganizationID()
	if m.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, m.OrganizationID, scope)
	}
	const q = `
		UPDATE membership
		SET status = $3, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND status = $4`
	tag, err := s.ttx.tx.Exec(ctx, q,
		m.ID.String(), scope.String(),
		string(domain.MembershipSuspended), string(domain.MembershipActive))
	if err != nil {
		return fmt.Errorf("postgres: gravação de suspensão de membership falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

// SaveActivation persists the acceptance of an invitation
// (domain.Membership.Activate already ran, so m.Status is active). The UPDATE
// only matches a row still invited, stamping activated_at: a membership
// resolved or revoked concurrently yields ErrNotInvited instead of being
// clobbered.
func (s *TenantMembershipStore) SaveActivation(ctx context.Context, m domain.Membership) error {
	scope := s.ttx.scope.OrganizationID()
	if m.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, m.OrganizationID, scope)
	}
	if m.Status != domain.MembershipActive {
		return fmt.Errorf("postgres: aceite exige membership ativo após a transição de domínio")
	}
	const q = `
		UPDATE membership
		SET status = $3, activated_at = now(), updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND status = $4`
	tag, err := s.ttx.tx.Exec(ctx, q,
		m.ID.String(), scope.String(),
		string(domain.MembershipActive), string(domain.MembershipInvited))
	if err != nil {
		return fmt.Errorf("postgres: gravação do aceite falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotInvited
	}
	return nil
}
