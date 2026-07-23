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
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrAuthCodeNotFound is returned when no authorization code matches a presented
// code's hash.
var ErrAuthCodeNotFound = errors.New("postgres: código de autorização não encontrado")

// AuthorizationCodeStore is the tenant-scoped store for authorization_code.
type AuthorizationCodeStore struct {
	ttx *TenantTx
}

// NewAuthorizationCodeStore builds the store on an open tenant transaction.
func NewAuthorizationCodeStore(ttx *TenantTx) *AuthorizationCodeStore {
	return &AuthorizationCodeStore{ttx: ttx}
}

// Create persists a freshly issued authorization code.
func (s *AuthorizationCodeStore) Create(ctx context.Context, c domain.AuthorizationCode) error {
	if c.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantGrant, c.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `
		INSERT INTO authorization_code (id, code_hash, client_id, redirect_uri, code_challenge, session_id, organization_id, scopes, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		c.ID.String(), c.CodeHash, c.ClientID, c.RedirectURI, c.CodeChallenge,
		c.SessionID.String(), c.OrganizationID.String(), c.Scopes, c.ExpiresAt); err != nil {
		return fmt.Errorf("postgres: criação de código de autorização falhou: %w", err)
	}
	return nil
}

// GetByHash loads a code by the hash of a presented secret, locking the row FOR
// UPDATE so redemption is serialized (a code is single-use).
func (s *AuthorizationCodeStore) GetByHash(ctx context.Context, hash []byte) (domain.AuthorizationCode, error) {
	const q = `
		SELECT id::text, code_hash, client_id, redirect_uri, code_challenge, session_id::text, organization_id::text, scopes, expires_at, used
		FROM authorization_code
		WHERE code_hash = $1 AND organization_id = $2
		FOR UPDATE`
	var c domain.AuthorizationCode
	var idText, sessText, orgText string
	err := s.ttx.tx.QueryRow(ctx, q, hash, s.ttx.scope.OrganizationID().String()).
		Scan(&idText, &c.CodeHash, &c.ClientID, &c.RedirectURI, &c.CodeChallenge, &sessText, &orgText, &c.Scopes, &c.ExpiresAt, &c.Used)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthorizationCode{}, ErrAuthCodeNotFound
	}
	if err != nil {
		return domain.AuthorizationCode{}, fmt.Errorf("postgres: leitura de código de autorização falhou: %w", err)
	}
	if c.ID, err = uuid.Parse(idText); err != nil {
		return domain.AuthorizationCode{}, fmt.Errorf("postgres: id inválido: %w", err)
	}
	if c.SessionID, err = uuid.Parse(sessText); err != nil {
		return domain.AuthorizationCode{}, fmt.Errorf("postgres: session_id inválido: %w", err)
	}
	if c.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.AuthorizationCode{}, fmt.Errorf("postgres: organização inválida: %w", err)
	}
	return c, nil
}

// MarkUsed flags a code as redeemed (single-use). It only succeeds while the code
// is still unused, so a concurrent double-redemption cannot both win.
func (s *AuthorizationCodeStore) MarkUsed(ctx context.Context, id uuid.UUID) error {
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE authorization_code SET used = true WHERE id = $1 AND organization_id = $2 AND used = false`,
		id.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return fmt.Errorf("postgres: marcação de código usado falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAuthCodeExpiredOrUsed
	}
	return nil
}

// ResolveAuthCodeOrg finds the organization an authorization code belongs to,
// keyed by the presented code's hash, under global-read (the org is unknown at
// token-exchange time; the code IS the capability).
func ResolveAuthCodeOrg(ctx context.Context, pool *pgxpool.Pool, presentedSecret string) (uuid.UUID, error) {
	var orgText string
	err := WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, 'on', true)", domain.RLSGlobalReadSettingName); err != nil {
			return fmt.Errorf("postgres: fixação de leitura global falhou: %w", err)
		}
		return tx.QueryRow(ctx,
			`SELECT organization_id::text FROM authorization_code WHERE code_hash = $1`,
			domain.HashAuthCode(presentedSecret)).Scan(&orgText)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrAuthCodeNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("postgres: resolução de org do código falhou: %w", err)
	}
	return uuid.Parse(orgText)
}
