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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthCodeIssuer implements the /authorize endpoint's code issuance (pacote 006
// wiring): it mints an authorization code for an authenticated session and stores
// it in the session's tenant. It returns the code secret to put in the redirect.
type AuthCodeIssuer struct {
	pool    *pgxpool.Pool
	codeTTL time.Duration
	now     func() time.Time
}

// NewAuthCodeIssuer builds the issuer. codeTTL is clamped to AuthCodeMaxTTL.
func NewAuthCodeIssuer(pool *pgxpool.Pool, codeTTL time.Duration) *AuthCodeIssuer {
	return &AuthCodeIssuer{pool: pool, codeTTL: codeTTL, now: time.Now}
}

// IssueCode mints and persists a code for the session, in the session's active
// tenant. The session must be active (ActiveTenant supplies the org).
func (i *AuthCodeIssuer) IssueCode(ctx context.Context, clientID, redirectURI, codeChallenge string, session *domain.AuthSession, scopes []string) (string, error) {
	if session == nil {
		return "", fmt.Errorf("authcode: sessão ausente")
	}
	_, org, err := session.ActiveTenant()
	if err != nil {
		return "", err
	}
	secret, code, err := domain.NewAuthorizationCode(clientID, redirectURI, codeChallenge, session.ID, org, scopes, i.codeTTL, i.now())
	if err != nil {
		return "", err
	}
	scope, err := domain.NewTenantScope(org)
	if err != nil {
		return "", err
	}
	if err := NewTenantRepository(i.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewAuthorizationCodeStore(ttx).Create(ctx, code)
	}); err != nil {
		return "", err
	}
	return secret, nil
}

// AuthCodeGrant implements the /token authorization_code grant: it resolves the
// code's org, redeems the code (single-use, PKCE, redirect check), issues a fresh
// access token AND a new refresh-token family for the session, atomically for the
// redemption+refresh side.
type AuthCodeGrant struct {
	pool       *pgxpool.Pool
	registry   *domain.ClientRegistry
	issuer     *TokenIssuer
	refreshTTL time.Duration
	now        func() time.Time
}

// NewAuthCodeGrant builds the grant.
func NewAuthCodeGrant(pool *pgxpool.Pool, registry *domain.ClientRegistry, issuer *TokenIssuer, refreshTTL time.Duration) *AuthCodeGrant {
	return &AuthCodeGrant{pool: pool, registry: registry, issuer: issuer, refreshTTL: refreshTTL, now: time.Now}
}

// Exchange redeems the code and returns the tokens.
func (g *AuthCodeGrant) Exchange(ctx context.Context, codeSecret, redirectURI, codeVerifier string) (domain.RefreshResult, error) {
	org, err := ResolveAuthCodeOrg(ctx, g.pool, codeSecret)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	scope, err := domain.NewTenantScope(org)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	repo := NewTenantRepository(g.pool, scope)
	now := g.now()

	var (
		sessionID     uuid.UUID
		clientID      string
		newRefreshSec string
	)
	err = repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		codeStore := NewAuthorizationCodeStore(ttx)
		code, err := codeStore.GetByHash(ctx, domain.HashAuthCode(codeSecret))
		if err != nil {
			return err
		}
		if err := code.Redeem(redirectURI, codeVerifier, now); err != nil {
			return err
		}
		if err := codeStore.MarkUsed(ctx, code.ID); err != nil {
			return err
		}
		// New refresh-token family for the session.
		secret, hash, err := domain.NewRefreshSecret()
		if err != nil {
			return err
		}
		rt, err := domain.NewRefreshFamily(code.SessionID, org, hash, now.Add(g.refreshTTL))
		if err != nil {
			return err
		}
		if err := NewRefreshTokenStore(ttx).Create(ctx, rt); err != nil {
			return err
		}
		sessionID = code.SessionID
		clientID = code.ClientID
		newRefreshSec = secret
		return nil
	})
	if err != nil {
		return domain.RefreshResult{}, err
	}

	client, err := g.registry.Lookup(clientID)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	accessToken, claims, err := g.issuer.IssueAccessToken(ctx, repo, sessionID, client.Audience, client.AllowedScopes, "", now)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	return domain.RefreshResult{
		AccessToken:     accessToken,
		RefreshToken:    newRefreshSec,
		ExpiresInSecond: int(claims.ExpiresAt - claims.IssuedAt),
	}, nil
}
