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

// ErrRefreshTokenNotFound is returned when no refresh token matches a presented
// secret's hash — an unknown/forged token.
var ErrRefreshTokenNotFound = errors.New("postgres: refresh token não encontrado")

// RefreshTokenStore is the tenant-scoped store for refresh_token families
// (pacote 006). Built on a TenantTx, it carries the organization_id predicate
// (Barreira 1) and the SET LOCAL tenant setting the RLS policy reads (Barreira 2).
type RefreshTokenStore struct {
	ttx *TenantTx
}

// NewRefreshTokenStore builds the store on an open tenant transaction.
func NewRefreshTokenStore(ttx *TenantTx) *RefreshTokenStore {
	return &RefreshTokenStore{ttx: ttx}
}

// Create persists a refresh token (new family or a rotation successor).
func (s *RefreshTokenStore) Create(ctx context.Context, r domain.RefreshToken) error {
	if r.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantGrant, r.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `
		INSERT INTO refresh_token (id, family_id, session_id, organization_id, token_hash, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		r.ID.String(), r.FamilyID.String(), r.SessionID.String(), r.OrganizationID.String(),
		r.TokenHash, string(r.Status), r.ExpiresAt); err != nil {
		return fmt.Errorf("postgres: criação de refresh token falhou: %w", err)
	}
	return nil
}

// GetByHash finds a refresh token by the hash of a presented secret, locking the
// row FOR UPDATE so the rotation/reuse decision is serialized against a
// concurrent exchange of the same token (the race a reuse attack rides).
func (s *RefreshTokenStore) GetByHash(ctx context.Context, hash []byte) (domain.RefreshToken, error) {
	const q = `
		SELECT id::text, family_id::text, session_id::text, organization_id::text, token_hash, status, expires_at
		FROM refresh_token
		WHERE token_hash = $1 AND organization_id = $2
		FOR UPDATE`
	var r domain.RefreshToken
	var idText, famText, sessText, orgText, status string
	err := s.ttx.tx.QueryRow(ctx, q, hash, s.ttx.scope.OrganizationID().String()).
		Scan(&idText, &famText, &sessText, &orgText, &r.TokenHash, &status, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, ErrRefreshTokenNotFound
	}
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres: leitura de refresh token falhou: %w", err)
	}
	if r.ID, err = uuid.Parse(idText); err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres: id inválido: %w", err)
	}
	if r.FamilyID, err = uuid.Parse(famText); err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres: family_id inválido: %w", err)
	}
	if r.SessionID, err = uuid.Parse(sessText); err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres: session_id inválido: %w", err)
	}
	if r.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres: organização inválida: %w", err)
	}
	r.Status = domain.RefreshTokenStatus(status)
	return r, nil
}

// SetStatus updates one token's status (e.g. active → rotated).
func (s *RefreshTokenStore) SetStatus(ctx context.Context, id uuid.UUID, status domain.RefreshTokenStatus) error {
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE refresh_token SET status = $3, updated_at = now() WHERE id = $1 AND organization_id = $2`,
		id.String(), s.ttx.scope.OrganizationID().String(), string(status))
	if err != nil {
		return fmt.Errorf("postgres: atualização de refresh token falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

// RevokeFamily revokes EVERY non-revoked token of a family — the cascade a reuse
// detection triggers (T-008 / spec "Reuso detectado"). Returns how many tokens
// were revoked. Idempotent.
func (s *RefreshTokenStore) RevokeFamily(ctx context.Context, familyID uuid.UUID) (int, error) {
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE refresh_token SET status = 'revoked', updated_at = now()
		 WHERE family_id = $1 AND organization_id = $2 AND status <> 'revoked'`,
		familyID.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return 0, fmt.Errorf("postgres: revogação de família de refresh falhou: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RevokeBySession revokes every non-revoked refresh token of a session — the
// refresh-token leg of the session revocation cascade (logout, membership
// revoke, grant expiry). Returns how many were revoked. Idempotent.
func (s *RefreshTokenStore) RevokeBySession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE refresh_token SET status = 'revoked', updated_at = now()
		 WHERE session_id = $1 AND organization_id = $2 AND status <> 'revoked'`,
		sessionID.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return 0, fmt.Errorf("postgres: revogação de refresh por sessão falhou: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
