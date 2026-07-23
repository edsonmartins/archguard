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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

type fakeCodeIssuer struct{ secret string }

func (f fakeCodeIssuer) IssueCode(context.Context, string, string, string, *domain.AuthSession, []string) (string, error) {
	return f.secret, nil
}

func authorizeReq(q url.Values) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), nil)
}

func warpgateAuthorizeParams() url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {"warpgate"},
		"redirect_uri":          {"https://warpgate.archgate.internal/@warpgate/oidc/callback"},
		"code_challenge":        {"a-challenge"},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile"},
		"state":                 {"xyz"},
	}
}

// Autorização válida com sessão autenticada: redireciona ao callback com o code.
func TestAuthorizeRedirectsWithCode(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	session := buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
	h := NewAuthorizeHandler(reg, staticResolver{session, true}, fakeCodeIssuer{secret: "ac_code123"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeReq(warpgateAuthorizeParams()))
	if rec.Code != http.StatusFound {
		t.Fatalf("code = %d, quero 302", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("code") != "ac_code123" || loc.Query().Get("state") != "xyz" {
		t.Fatalf("redirect deveria trazer code e state: %s", rec.Header().Get("Location"))
	}
}

// PKCE ausente: redireciona com error=invalid_request.
func TestAuthorizeRejectsMissingPKCE(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	session := buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
	h := NewAuthorizeHandler(reg, staticResolver{session, true}, fakeCodeIssuer{secret: "x"})

	p := warpgateAuthorizeParams()
	p.Del("code_challenge")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeReq(p))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("error") != "invalid_request" {
		t.Fatalf("PKCE ausente deveria redirecionar com invalid_request: %s", rec.Header().Get("Location"))
	}
}

// Cliente desconhecido / redirect não registrado: NÃO redireciona (open-redirect).
func TestAuthorizeRejectsUnknownClientDirectly(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	h := NewAuthorizeHandler(reg, staticResolver{nil, false}, fakeCodeIssuer{})

	p := warpgateAuthorizeParams()
	p.Set("client_id", "desconhecido")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeReq(p))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cliente desconhecido deveria ser recusado diretamente (400), veio %d", rec.Code)
	}

	// redirect_uri não registrado também.
	p2 := warpgateAuthorizeParams()
	p2.Set("redirect_uri", "https://evil/cb")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, authorizeReq(p2))
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("redirect não registrado deveria ser recusado diretamente (400), veio %d", rec2.Code)
	}
}

// Sem sessão autenticada: redireciona com login_required.
func TestAuthorizeLoginRequired(t *testing.T) {
	reg, _ := domain.DefaultClientRegistry()
	h := NewAuthorizeHandler(reg, staticResolver{nil, false}, fakeCodeIssuer{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeReq(warpgateAuthorizeParams()))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("error") != "login_required" {
		t.Fatalf("sem sessão deveria ser login_required: %s", rec.Header().Get("Location"))
	}
}
