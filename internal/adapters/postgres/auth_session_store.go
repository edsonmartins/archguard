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
)

// Errors of the auth_session stores.
var (
	// ErrSessionNotFound is returned when the session does not exist WITHIN the
	// store's scope — a session of another identity is indistinguishable from a
	// missing one, on purpose.
	ErrSessionNotFound = errors.New("postgres: sessão não encontrada")
	// ErrCrossIdentityWrite is returned when an identity-scoped store is asked to
	// persist a session of a different identity — the identity-axis analogue of
	// ErrCrossTenantWrite.
	ErrCrossIdentityWrite = errors.New("postgres: escrita de sessão de outra identidade recusada")
	// ErrSessionNotPending is returned when persisting a tenant selection for a
	// session that is no longer pending (already resolved elsewhere, or revoked).
	ErrSessionNotPending = errors.New("postgres: sessão não está pendente de seleção de tenant")
)

// IdentitySessionStore is the login-flow store for auth_session rows: create the
// session at authentication, persist the explicit tenant selection, read and
// revoke the identity's own sessions. It is scoped to ONE identity by
// construction (domain.IdentityScope — no constructor without it): every query
// carries an identity_id predicate, so a session of another identity is
// unreachable. This is Barreira 1 on the identity axis; pending rows have no
// tenant yet, so the tenant axis cannot scope them. (RLS for auth_session is
// deferred to T-012 — see migration 0012.)
type IdentitySessionStore struct {
	q     Querier
	scope domain.IdentityScope
}

// NewIdentitySessionStore builds the store over any Querier (pool or
// transaction), bound to an identity scope.
func NewIdentitySessionStore(q Querier, scope domain.IdentityScope) *IdentitySessionStore {
	return &IdentitySessionStore{q: q, scope: scope}
}

// Create persists a freshly-resolved session (active or pending_selection). It
// refuses a session of another identity (ErrCrossIdentityWrite). Timestamps are
// stamped by the database.
func (s *IdentitySessionStore) Create(ctx context.Context, as domain.AuthSession) error {
	if as.IdentityID != s.scope.IdentityID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossIdentityWrite, as.IdentityID, s.scope.IdentityID())
	}
	const q = `
		INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.q.Exec(ctx, q,
		as.ID.String(), as.IdentityID.String(),
		uuidTextOrNil(as.MembershipID), uuidTextOrNil(as.OrganizationID),
		string(as.Status), string(as.ProvenAAL))
	if err != nil {
		return fmt.Errorf("postgres: criação de auth_session falhou: %w", err)
	}
	return nil
}

// Get returns one session of the store's identity. A session of another
// identity yields ErrSessionNotFound — the predicate makes it unreachable.
func (s *IdentitySessionStore) Get(ctx context.Context, sessionID uuid.UUID) (domain.AuthSession, error) {
	const q = `
		SELECT id::text, identity_id::text, membership_id::text, organization_id::text,
		       status, proven_aal, revoked_at, created_at, updated_at
		FROM auth_session
		WHERE id = $1 AND identity_id = $2`
	row := s.q.QueryRow(ctx, q, sessionID.String(), s.scope.IdentityID().String())
	as, err := scanAuthSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthSession{}, ErrSessionNotFound
	}
	return as, err
}

// SaveSelection persists the explicit tenant selection of a pending session
// (domain.AuthSession.SelectTenant already ran, so as.Status is active). The
// UPDATE only matches a row still pending_selection: a session resolved or
// revoked concurrently yields ErrSessionNotPending instead of being clobbered.
func (s *IdentitySessionStore) SaveSelection(ctx context.Context, as domain.AuthSession) error {
	if as.IdentityID != s.scope.IdentityID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossIdentityWrite, as.IdentityID, s.scope.IdentityID())
	}
	if as.Status != domain.SessionActive || as.MembershipID == nil || as.OrganizationID == nil {
		return fmt.Errorf("postgres: seleção de tenant exige sessão ativa com membership resolvido")
	}
	const q = `
		UPDATE auth_session
		SET membership_id = $3, organization_id = $4, status = $5, updated_at = now()
		WHERE id = $1 AND identity_id = $2 AND status = 'pending_selection'`
	tag, err := s.q.Exec(ctx, q,
		as.ID.String(), as.IdentityID.String(),
		as.MembershipID.String(), as.OrganizationID.String(), string(domain.SessionActive))
	if err != nil {
		return fmt.Errorf("postgres: gravação da seleção de tenant falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotPending
	}
	return nil
}

// Revoke terminates one session of the store's identity. Idempotent: revoking a
// revoked session keeps the original revoked_at. A session of another identity
// yields ErrSessionNotFound.
func (s *IdentitySessionStore) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	const q = `
		UPDATE auth_session
		SET status = 'revoked', revoked_at = COALESCE(revoked_at, now()), updated_at = now()
		WHERE id = $1 AND identity_id = $2`
	tag, err := s.q.Exec(ctx, q, sessionID.String(), s.scope.IdentityID().String())
	if err != nil {
		return fmt.Errorf("postgres: revogação de auth_session falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// TenantSessionStore is the tenant-side view of auth_session: the active
// sessions WITHIN one organization (tenant management, and the membership
// revocation cascade of T-014). Built on a TenantTx, so it carries the explicit
// organization_id predicate (Barreira 1) and the SET LOCAL tenant setting the
// transaction already applied.
type TenantSessionStore struct {
	ttx *TenantTx
}

// NewTenantSessionStore builds the store on an open tenant transaction. There
// is no constructor without a tenant.
func NewTenantSessionStore(ttx *TenantTx) *TenantSessionStore {
	return &TenantSessionStore{ttx: ttx}
}

// ListActive returns the active sessions whose active tenant is this store's
// organization. Pending sessions have no tenant and never appear here.
func (s *TenantSessionStore) ListActive(ctx context.Context) ([]domain.AuthSession, error) {
	const q = `
		SELECT id::text, identity_id::text, membership_id::text, organization_id::text,
		       status, proven_aal, revoked_at, created_at, updated_at
		FROM auth_session
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY created_at`
	rows, err := s.ttx.tx.Query(ctx, q, s.ttx.scope.OrganizationID().String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de auth_session falhou: %w", err)
	}
	defer rows.Close()

	var out []domain.AuthSession
	for rows.Next() {
		as, err := scanAuthSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, as)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de auth_session falhou: %w", err)
	}
	return out, nil
}

// scanAuthSession reads one auth_session row (see the SELECT column order the
// stores share). UUIDs travel as text, nullable ones as *string.
func scanAuthSession(row pgx.Row) (domain.AuthSession, error) {
	var as domain.AuthSession
	var idText, idnText, status, aal string
	var memText, orgText *string
	if err := row.Scan(&idText, &idnText, &memText, &orgText,
		&status, &aal, &as.RevokedAt, &as.CreatedAt, &as.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AuthSession{}, err
		}
		return domain.AuthSession{}, fmt.Errorf("postgres: leitura de auth_session falhou: %w", err)
	}
	var err error
	if as.ID, err = uuid.Parse(idText); err != nil {
		return domain.AuthSession{}, fmt.Errorf("postgres: id de auth_session inválido %q: %w", idText, err)
	}
	if as.IdentityID, err = uuid.Parse(idnText); err != nil {
		return domain.AuthSession{}, fmt.Errorf("postgres: identity_id inválido %q: %w", idnText, err)
	}
	if as.MembershipID, err = parseOptionalUUID("membership_id", memText); err != nil {
		return domain.AuthSession{}, err
	}
	if as.OrganizationID, err = parseOptionalUUID("organization_id", orgText); err != nil {
		return domain.AuthSession{}, err
	}
	as.Status = domain.SessionStatus(status)
	as.ProvenAAL = domain.AAL(aal)
	return as, nil
}

// uuidTextOrNil renders an optional UUID as a nullable text parameter.
func uuidTextOrNil(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	t := u.String()
	return &t
}

// parseOptionalUUID parses a nullable uuid::text column.
func parseOptionalUUID(col string, text *string) (*uuid.UUID, error) {
	if text == nil {
		return nil, nil
	}
	u, err := uuid.Parse(*text)
	if err != nil {
		return nil, fmt.Errorf("postgres: %s inválido %q: %w", col, *text, err)
	}
	return &u, nil
}
