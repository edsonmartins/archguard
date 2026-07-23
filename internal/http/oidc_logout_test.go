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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeEndSession struct {
	called bool
	err    error
}

func (f *fakeEndSession) EndSession(context.Context, uuid.UUID, string) error {
	f.called = true
	return f.err
}

// Logout com sessão: encerra e redireciona a um post_logout_redirect_uri
// REGISTRADO.
func TestEndSessionHandlerRedirects(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	session := buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
	ender := &fakeEndSession{}
	h := NewEndSessionHandler(staticResolver{session, true}, ender, reg)

	// warpgate tem esse redirect registrado.
	req := httptest.NewRequest(http.MethodGet, "/logout?client_id=warpgate&post_logout_redirect_uri=https://warpgate.archgate.internal/@warpgate/oidc/callback&state=s1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !ender.called {
		t.Fatalf("a sessão deveria ter sido encerrada")
	}
	if rec.Code != http.StatusFound {
		t.Fatalf("code = %d, quero 302", rec.Code)
	}
}

// Post-logout redirect NÃO registrado: não redireciona (204).
func TestEndSessionHandlerRefusesUnregisteredRedirect(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	session := buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
	h := NewEndSessionHandler(staticResolver{session, true}, &fakeEndSession{}, reg)

	req := httptest.NewRequest(http.MethodGet, "/logout?client_id=warpgate&post_logout_redirect_uri=https://evil/cb", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("redirect não registrado deveria dar 204, veio %d", rec.Code)
	}
}

// Falha na revogação local: 500 (fail-closed).
func TestEndSessionHandlerFailClosed(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	session := buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
	h := NewEndSessionHandler(staticResolver{session, true}, &fakeEndSession{err: errors.New("db")}, reg)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logout", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("falha de revogação deveria dar 500, veio %d", rec.Code)
	}
}
