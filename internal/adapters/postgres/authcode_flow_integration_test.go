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
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/oidc"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Fluxo Authorization Code + PKCE ponta a ponta: emite código para uma sessão,
// troca com o code_verifier correto por access+refresh; o mesmo código não pode
// ser reusado (uso único), e um code_verifier errado é recusado.
func TestAuthCodeFlowEndToEnd(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "authcode")

	at := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	sessID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, auth_methods)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5,ARRAY['password','totp']::text[])`,
		sessID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), at); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}
	session, err := getSession(pool, fx.scopeIdn, sessID)
	if err != nil {
		// getSession usa scopeIdn (fx.identity); a sessão é de fx.identity/memA.
		t.Fatalf("getSession: %v", err)
	}

	verifier := "verifier-de-teste-com-entropia-suficiente-abcdef123456"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	redirect := "https://warpgate.archgate.internal/@warpgate/oidc/callback"

	reg, _ := domain.DefaultClientRegistry()
	key, _ := oidc.GenerateSigningKey("kid-1")
	signer, _ := oidc.NewSigner(key)
	issuer := NewTokenIssuer(signer, "https://archguard.example", 10*time.Minute)

	// /authorize: emite o código.
	codeIssuer := NewAuthCodeIssuer(pool, 60*time.Second)
	secret, err := codeIssuer.IssueCode(ctx, "warpgate", redirect, challenge, &session, []string{"openid", "profile"})
	if err != nil {
		t.Fatalf("IssueCode: %v", err)
	}

	grant := NewAuthCodeGrant(pool, reg, issuer, 2*time.Hour)

	// code_verifier errado: recusado.
	if _, err := grant.Exchange(ctx, secret, redirect, "verifier-errado"); !errors.Is(err, domain.ErrPKCEVerificationFailed) {
		t.Fatalf("PKCE errado deveria ser recusado: %v", err)
	}

	// Troca correta.
	res, err := grant.Exchange(ctx, secret, redirect, verifier)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	claims, err := signer.Verify(res.AccessToken)
	if err != nil {
		t.Fatalf("o access token deveria verificar: %v", err)
	}
	if claims.Audience != "warpgate" || claims.Organization != fx.orgA.String() || res.RefreshToken == "" {
		t.Fatalf("tokens do code grant inesperados: %+v / %+v", claims, res)
	}

	// Uso único: o mesmo código não pode ser trocado de novo.
	if _, err := grant.Exchange(ctx, secret, redirect, verifier); !errors.Is(err, domain.ErrAuthCodeExpiredOrUsed) {
		t.Fatalf("código já usado deveria ser recusado: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM refresh_token WHERE session_id = $1", sessID.String())
		_, _ = pool.Exec(bg, "DELETE FROM authorization_code WHERE session_id = $1", sessID.String())
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE id = $1", sessID.String())
	})
}
