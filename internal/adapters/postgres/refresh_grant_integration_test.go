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
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/alerting"
	"github.com/casdoor/casdoor/internal/adapters/oidc"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Fluxo completo do grant refresh_token: renova (access verificável + refresh
// novo) e, no reuso do anterior, sinaliza reuso (família revogada).
func TestRefreshGrantEndToEnd(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "rgrant")

	at := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	sessID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, auth_methods)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5,ARRAY['password','totp']::text[])`,
		sessID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), at); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}

	repo := NewTenantRepository(pool, fx.tenantScopeA)
	secret1, hash1, _ := domain.NewRefreshSecret()
	rt, _ := domain.NewRefreshFamily(sessID, fx.orgA, hash1, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewRefreshTokenStore(ttx).Create(ctx, rt)
	}); err != nil {
		t.Fatalf("cria refresh: %v", err)
	}

	key, _ := oidc.GenerateSigningKey("kid-1")
	signer, _ := oidc.NewSigner(key)
	issuer := NewTokenIssuer(signer, "https://archguard.example", 10*time.Minute)
	grant := NewRefreshGrant(pool, NewAuditWriter(pool, fixedClock()), alerting.NewMemoryAlerter(),
		issuer, "warpgate", []string{"openid"}, 2*time.Hour)

	// Renovação normal.
	res, err := grant.Refresh(ctx, secret1)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.RefreshToken == secret1 || res.AccessToken == "" || res.ExpiresInSecond != 600 {
		t.Fatalf("resultado de refresh inesperado: %+v", res)
	}
	// O access token reemitido verifica e carrega os claims do tenant ativo.
	claims, err := signer.Verify(res.AccessToken)
	if err != nil {
		t.Fatalf("o access token reemitido deveria verificar: %v", err)
	}
	if claims.Organization != fx.orgA.String() || claims.Audience != "warpgate" || claims.ACR != "L2" {
		t.Fatalf("claims do access token reemitido inesperados: %+v", claims)
	}

	// Reuso do refresh anterior: sinaliza reuso.
	if _, err := grant.Refresh(ctx, secret1); !errors.Is(err, domain.ErrRefreshReuse) {
		t.Fatalf("reuso do refresh anterior deveria ser detectado: %v", err)
	}

	// Token desconhecido: not found.
	if _, err := grant.Refresh(ctx, "rt_inexistente"); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("token desconhecido deveria ser not found: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM refresh_token WHERE session_id = $1", sessID.String())
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE id = $1", sessID.String())
	})
}
