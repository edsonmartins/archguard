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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/oidc"
	"github.com/casdoor/casdoor/internal/domain"
)

// O mux roteia os endpoints OIDC (descoberta e JWKS respondem 200).
func TestOIDCServerRoutes(t *testing.T) {
	key, _ := oidc.GenerateSigningKey("kid-1")
	signer, _ := oidc.NewSigner(key)
	reg, _ := domain.DefaultClientRegistry()

	server := &OIDCServer{
		Discovery:  NewDiscoveryHandler(testDiscovery(t)),
		JWKS:       NewJWKSHandler(signer),
		Authorize:  NewAuthorizeHandler(reg, staticResolver{nil, false}, fakeCodeIssuer{}),
		Token:      NewTokenHandler(nil, fakeRefreshGrant{}),
		Introspect: NewIntrospectionHandler(signer, staticLiveness{live: false}),
		EndSession: NewEndSessionHandler(staticResolver{nil, false}, &fakeEndSession{}, reg),
	}
	h := server.Handler()

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/.well-known/openid-configuration", http.StatusOK},
		{http.MethodGet, "/jwks", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Fatalf("%s %s = %d, quero %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}

	// Um caminho não montado dá 404.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inexistente", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("caminho desconhecido deveria dar 404, veio %d", rec.Code)
	}
}
