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

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/oidc"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type staticLiveness struct{ live bool }

func (l staticLiveness) Live(context.Context, string, string) (bool, error) { return l.live, nil }

// signedIntrospectableToken builds a valid signed token and its signer.
func signedIntrospectableToken(t *testing.T) (*oidc.Signer, string) {
	t.Helper()
	id, org := uuid.New(), uuid.New()
	m, _ := domain.NewMembership(id, org)
	s, _ := domain.NewAuthSession(id, domain.AAL2, []domain.Membership{m})
	_ = s.SetAuthContext(time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC), []domain.FactorType{domain.FactorPassword, domain.FactorTOTP})
	claims, err := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
		Issuer: "iss", Audience: "warpgate", Subject: "sub", Session: &s,
		IssuedAt: time.Date(2026, 7, 23, 10, 1, 0, 0, time.UTC), AccessTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	key, _ := oidc.GenerateSigningKey("kid-1")
	signer, _ := oidc.NewSigner(key)
	token, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return signer, token
}

func introspect(t *testing.T, h *IntrospectionHandler, token string) domain.IntrospectionResponse {
	t.Helper()
	body := url.Values{"token": {token}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, quero 200", rec.Code)
	}
	var resp domain.IntrospectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

// Sessão viva -> active com claims. Sessão revogada -> active:false.
func TestIntrospectionHandler(t *testing.T) {
	signer, token := signedIntrospectableToken(t)

	// Fixar o relógio dentro da janela do token (senão expira vs relógio real).
	hLive := NewIntrospectionHandler(signer, staticLiveness{live: true})
	hLive.now = func() time.Time { return time.Date(2026, 7, 23, 10, 2, 0, 0, time.UTC) }
	if resp := introspect(t, hLive, token); !resp.Active || resp.ACR != "L2" {
		t.Fatalf("token vivo deveria ser active com acr L2: %+v", resp)
	}

	// Sessão revogada.
	hDead := NewIntrospectionHandler(signer, staticLiveness{live: false})
	hDead.now = func() time.Time { return time.Date(2026, 7, 23, 10, 2, 0, 0, time.UTC) }
	if resp := introspect(t, hDead, token); resp.Active || resp.Subject != "" {
		t.Fatalf("sessão revogada deveria ser active:false sem claims: %+v", resp)
	}
}

// Token não verificável (assinatura de outra chave) -> active:false, sem erro.
func TestIntrospectionUnverifiableToken(t *testing.T) {
	signer, _ := signedIntrospectableToken(t)
	otherKey, _ := oidc.GenerateSigningKey("kid-other")
	otherSigner, _ := oidc.NewSigner(otherKey)
	_, forged := signedIntrospectableTokenWith(t, otherSigner)

	h := NewIntrospectionHandler(signer, staticLiveness{live: true})
	if resp := introspect(t, h, forged); resp.Active {
		t.Fatalf("token de outra chave deveria ser active:false")
	}
}

func signedIntrospectableTokenWith(t *testing.T, signer *oidc.Signer) (*oidc.Signer, string) {
	t.Helper()
	id, org := uuid.New(), uuid.New()
	m, _ := domain.NewMembership(id, org)
	s, _ := domain.NewAuthSession(id, domain.AAL2, []domain.Membership{m})
	_ = s.SetAuthContext(time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC), []domain.FactorType{domain.FactorTOTP})
	claims, _ := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
		Issuer: "iss", Audience: "warpgate", Subject: "sub", Session: &s,
		IssuedAt: time.Date(2026, 7, 23, 10, 1, 0, 0, time.UTC), AccessTTL: 10 * time.Minute,
	})
	token, _ := signer.Sign(claims)
	return signer, token
}
