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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshGrant implements the token endpoint's refresh_token grant (pacote 006
// wiring): it resolves the token's organization, exchanges (rotates) the refresh
// token — revoking the family on reuse — and mints a fresh access token for the
// session. It composes ResolveRefreshTokenOrg + RefreshExchanger + TokenIssuer,
// with a fixed refresh TTL and the component audience/scopes.
type RefreshGrant struct {
	pool       *pgxpool.Pool
	audit      AuditEmitter
	alerter    domain.Alerter
	issuer     *TokenIssuer
	audience   string
	scopes     []string
	refreshTTL time.Duration
	now        func() time.Time
}

// NewRefreshGrant builds the grant. audience/scopes are the component's; refreshTTL
// is the tenant refresh lifetime.
func NewRefreshGrant(pool *pgxpool.Pool, audit AuditEmitter, alerter domain.Alerter, issuer *TokenIssuer, audience string, scopes []string, refreshTTL time.Duration) *RefreshGrant {
	return &RefreshGrant{
		pool: pool, audit: audit, alerter: alerter, issuer: issuer,
		audience: audience, scopes: scopes, refreshTTL: refreshTTL, now: time.Now,
	}
}

// Refresh resolves the org, rotates the refresh token and issues a new access
// token. It surfaces domain.ErrRefreshReuse on reuse (family already revoked) and
// ErrRefreshTokenNotFound for an unknown token.
func (g *RefreshGrant) Refresh(ctx context.Context, presentedSecret string) (domain.RefreshResult, error) {
	org, err := ResolveRefreshTokenOrg(ctx, g.pool, presentedSecret)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	scope, err := domain.NewTenantScope(org)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	repo := NewTenantRepository(g.pool, scope)

	now := g.now()
	exchanger := NewRefreshExchanger(repo, g.audit, g.alerter)
	res, err := exchanger.Exchange(ctx, presentedSecret, now.Add(g.refreshTTL))
	if err != nil {
		return domain.RefreshResult{}, err // ErrRefreshReuse / not found
	}

	accessToken, claims, err := g.issuer.IssueAccessToken(ctx, repo, res.NewToken.SessionID, g.audience, g.scopes, "", now)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	return domain.RefreshResult{
		AccessToken:     accessToken,
		RefreshToken:    res.NewSecret,
		ExpiresInSecond: int(claims.ExpiresAt - claims.IssuedAt),
	}, nil
}
