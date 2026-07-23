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
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccessTokenSigner signs the OIDC claims into an access token — the oidc.Signer
// implements it.
type AccessTokenSigner interface {
	Sign(claims domain.OIDCClaims) (string, error)
}

// TokenIssuer mints a signed access token for an authenticated session (pacote
// 006 wiring): it loads the session and the identity's opaque subject, builds the
// v1 claim set for the requested audience/scopes, and signs it. It is the shared
// issuance path of the authorization-code and refresh-token grants.
type TokenIssuer struct {
	signer    AccessTokenSigner
	issuer    string
	accessTTL time.Duration
}

// NewTokenIssuer builds the issuer. accessTTL must lie within the contract's 5–15
// min band (enforced by the claims builder).
func NewTokenIssuer(signer AccessTokenSigner, issuer string, accessTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{signer: signer, issuer: issuer, accessTTL: accessTTL}
}

// IssueAccessToken loads the session sessionID within the tenant repo's org,
// resolves the identity subject, builds the claims for the audience and granted
// scopes, and returns the signed access token plus its claims. It refuses a
// session that is not active (no token for a pending/revoked session).
func (i *TokenIssuer) IssueAccessToken(ctx context.Context, repo *TenantRepository, sessionID uuid.UUID, audience string, grantedScopes []string, email string, now time.Time) (string, domain.OIDCClaims, error) {
	var (
		session domain.AuthSession
		subject string
	)
	err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		const q = `
			SELECT id::text, identity_id::text, membership_id::text, organization_id::text,
			       status, proven_aal, token_generation, auth_time, auth_methods, enrollment_required, revoked_at, created_at, updated_at
			FROM auth_session
			WHERE id = $1 AND organization_id = $2`
		s, err := scanAuthSession(ttx.Tx().QueryRow(ctx, q, sessionID.String(), repo.Scope().OrganizationID().String()))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionNotFound
		}
		if err != nil {
			return err
		}
		session = s
		return ttx.Tx().QueryRow(ctx, `SELECT subject FROM identity WHERE id = $1`, s.IdentityID.String()).Scan(&subject)
	})
	if err != nil {
		return "", domain.OIDCClaims{}, err
	}

	claims, err := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
		Issuer:        i.issuer,
		Audience:      audience,
		Subject:       subject,
		Session:       &session,
		IssuedAt:      now,
		AccessTTL:     i.accessTTL,
		GrantedScopes: grantedScopes,
		Email:         email,
	})
	if err != nil {
		return "", domain.OIDCClaims{}, err
	}
	token, err := i.signer.Sign(claims)
	if err != nil {
		return "", domain.OIDCClaims{}, err
	}
	return token, claims, nil
}

// ResolveRefreshTokenOrg finds the organization a refresh token belongs to,
// keyed by the hash of the presented secret. The lookup runs under global-read
// (the read RLS policy of refresh_token admits app.global_read='on'), because at
// token-exchange time the org is not yet known — the presented secret IS the
// capability. It returns ErrRefreshTokenNotFound for an unknown token.
func ResolveRefreshTokenOrg(ctx context.Context, pool *pgxpool.Pool, presentedSecret string) (uuid.UUID, error) {
	var orgText string
	err := WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, 'on', true)", domain.RLSGlobalReadSettingName); err != nil {
			return fmt.Errorf("postgres: fixação de leitura global falhou: %w", err)
		}
		return tx.QueryRow(ctx,
			`SELECT organization_id::text FROM refresh_token WHERE token_hash = $1`,
			domain.HashRefreshToken(presentedSecret)).Scan(&orgText)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrRefreshTokenNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("postgres: resolução de org do refresh falhou: %w", err)
	}
	return uuid.Parse(orgText)
}
