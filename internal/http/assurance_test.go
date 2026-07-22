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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// staticResolver returns a fixed session (or none), standing in for the
// authentication layer.
type staticResolver struct {
	session *domain.AuthSession
	present bool
}

func (r staticResolver) Session(*http.Request) (*domain.AuthSession, bool) {
	return r.session, r.present
}

func buildSession(t *testing.T, aal domain.AAL, methods ...domain.FactorType) *domain.AuthSession {
	t.Helper()
	id := uuid.New()
	org := uuid.New()
	m, err := domain.NewMembership(id, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	s, err := domain.NewAuthSession(id, aal, []domain.Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := s.SetAuthContext(time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC), methods); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	return &s
}

func testGuard(t *testing.T) *domain.AssuranceGuard {
	t.Helper()
	cat := domain.NewOperationCatalog()
	if err := cat.Register(domain.Operation{ID: "audit.export", Level: domain.L3}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return domain.NewAssuranceGuard(cat)
}

func nextOK() (http.Handler, *bool) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

// Sessão AAL3 WebAuthn passa: o handler protegido roda.
func TestAssuranceMiddlewareAllows(t *testing.T) {
	mw := NewAssuranceMiddleware(testGuard(t), staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true})
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("audit.export", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/verify", nil))
	if rec.Code != http.StatusOK || !*called {
		t.Fatalf("sessão suficiente deveria rodar o handler: code=%d called=%v", rec.Code, *called)
	}
}

// TOTP AAL2 numa operação L3: 401 com desafio de step-up (RFC 9470) informando
// acr_values=aal3, e o handler NÃO roda.
func TestAssuranceMiddlewareChallengesInsufficient(t *testing.T) {
	mw := NewAssuranceMiddleware(testGuard(t), staticResolver{buildSession(t, domain.AAL2, domain.FactorTOTP), true})
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("audit.export", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/verify", nil))

	if *called {
		t.Fatalf("handler protegido não deveria rodar em garantia insuficiente")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, quero 401", rec.Code)
	}
	wa := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(wa, "insufficient_user_authentication") || !strings.Contains(wa, `acr_values="aal3"`) {
		t.Fatalf("WWW-Authenticate deveria trazer o desafio de step-up com acr_values=aal3: %q", wa)
	}
	var body assuranceErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decodifica corpo: %v", err)
	}
	if body.RequiredLevel != "L3" || body.ACRValues != "aal3" || body.NeedsPhishingResistant == nil || !*body.NeedsPhishingResistant {
		t.Fatalf("corpo deveria informar L3/aal3/phishing-resistant: %+v", body)
	}
}

// Sem sessão: também é desafiado (nunca liberado).
func TestAssuranceMiddlewareNoSession(t *testing.T) {
	mw := NewAssuranceMiddleware(testGuard(t), staticResolver{nil, false})
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("audit.export", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/verify", nil))
	if *called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão deveria ser desafiado: code=%d called=%v", rec.Code, *called)
	}
}

// Operação não classificada: defeito de fiação — 500, e o handler NÃO roda
// (fail-closed).
func TestAssuranceMiddlewareUnclassified(t *testing.T) {
	mw := NewAssuranceMiddleware(testGuard(t), staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true})
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("session.open", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if *called || rec.Code != http.StatusInternalServerError {
		t.Fatalf("op não classificada deveria negar com 500: code=%d called=%v", rec.Code, *called)
	}
}
