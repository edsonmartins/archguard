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
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

var (
	fixtureAuthTime = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	fixtureNow      = fixtureAuthTime.Add(2 * time.Minute) // dentro da janela L3
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
	if err := s.SetAuthContext(fixtureAuthTime, methods); err != nil {
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
	if err := cat.Register(domain.Operation{ID: "profile.read", Level: domain.L1}); err != nil {
		t.Fatalf("Register L1: %v", err)
	}
	return domain.NewAssuranceGuard(cat)
}

// staticFloor is a fixed tenant MFA floor, standing in for the org policy
// authority.
type staticFloor struct {
	aal domain.AAL
	err error
}

func (f staticFloor) RequiredAAL(context.Context, uuid.UUID) (domain.AAL, error) {
	return f.aal, f.err
}

// middlewareAt builds a middleware whose clock is pinned to now, with no extra
// tenant floor (AAL1).
func middlewareAt(t *testing.T, resolve SessionResolver, now time.Time) *AssuranceMiddleware {
	t.Helper()
	return middlewareWithFloor(t, resolve, staticFloor{aal: domain.AAL1}, now)
}

// middlewareWithFloor builds a middleware with an explicit tenant-floor policy.
func middlewareWithFloor(t *testing.T, resolve SessionResolver, floor TenantFloor, now time.Time) *AssuranceMiddleware {
	t.Helper()
	mw := NewAssuranceMiddleware(testGuard(t), resolve, floor)
	mw.now = func() time.Time { return now }
	return mw
}

func nextOK() (http.Handler, *bool) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

// Sessão AAL3 WebAuthn recente passa: o handler protegido roda.
func TestAssuranceMiddlewareAllows(t *testing.T) {
	mw := middlewareAt(t, staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true}, fixtureNow)
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
	mw := middlewareAt(t, staticResolver{buildSession(t, domain.AAL2, domain.FactorTOTP), true}, fixtureNow)
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

// Sessão AAL3 WebAuthn, mas antiga (além da janela L3): também é desafiada a
// reautenticar — o handler NÃO roda (cenário "Operação L3 com sessão antiga").
func TestAssuranceMiddlewareChallengesStale(t *testing.T) {
	staleNow := fixtureAuthTime.Add(10 * time.Minute)
	mw := middlewareAt(t, staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true}, staleNow)
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("audit.export", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/verify", nil))
	if *called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("sessão antiga deveria ser desafiada: code=%d called=%v", rec.Code, *called)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), `acr_values="aal3"`) {
		t.Fatalf("desafio de sessão antiga deveria trazer acr_values=aal3")
	}
}

// Sem sessão: também é desafiado (nunca liberado).
func TestAssuranceMiddlewareNoSession(t *testing.T) {
	mw := middlewareAt(t, staticResolver{nil, false}, fixtureNow)
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("audit.export", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/verify", nil))
	if *called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão deveria ser desafiado: code=%d called=%v", rec.Code, *called)
	}
}

// Precedência "mais restritiva vence" (T-011): num tenant com piso AAL3, uma
// operação L1 exige WebAuthn — a sessão TOTP AAL2 é desafiada com acr_values=aal3.
func TestAssuranceMiddlewareTenantFloorRaises(t *testing.T) {
	mw := middlewareWithFloor(t,
		staticResolver{buildSession(t, domain.AAL2, domain.FactorTOTP), true},
		staticFloor{aal: domain.AAL3}, fixtureNow)
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("profile.read", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if *called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("piso AAL3 deveria desafiar L1 com sessão AAL2: code=%d called=%v", rec.Code, *called)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), `acr_values="aal3"`) {
		t.Fatalf("desafio deveria trazer acr_values=aal3 pelo piso do tenant")
	}
}

// Falha ao resolver a política do tenant é fail-closed: 500, handler não roda.
func TestAssuranceMiddlewarePolicyUnavailable(t *testing.T) {
	mw := middlewareWithFloor(t,
		staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true},
		staticFloor{err: context.DeadlineExceeded}, fixtureNow)
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("profile.read", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if *called || rec.Code != http.StatusInternalServerError {
		t.Fatalf("política indisponível deveria negar com 500: code=%d called=%v", rec.Code, *called)
	}
}

// Operação não classificada: defeito de fiação — 500, e o handler NÃO roda.
func TestAssuranceMiddlewareUnclassified(t *testing.T) {
	mw := middlewareAt(t, staticResolver{buildSession(t, domain.AAL3, domain.FactorWebAuthn), true}, fixtureNow)
	next, called := nextOK()
	rec := httptest.NewRecorder()
	mw.Require("session.open", next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if *called || rec.Code != http.StatusInternalServerError {
		t.Fatalf("op não classificada deveria negar com 500: code=%d called=%v", rec.Code, *called)
	}
}
