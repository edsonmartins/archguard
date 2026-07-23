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
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/oidc"
	"github.com/casdoor/casdoor/internal/domain"
)

func testDiscovery(t *testing.T) domain.DiscoveryDocument {
	t.Helper()
	doc, err := domain.BuildDiscoveryDocument("https://archguard.example", domain.OIDCEndpoints{
		Authorization: "https://archguard.example/authorize",
		Token:         "https://archguard.example/token",
		JWKS:          "https://archguard.example/jwks",
		Introspection: "https://archguard.example/introspect",
		EndSession:    "https://archguard.example/logout",
	})
	if err != nil {
		t.Fatalf("BuildDiscoveryDocument: %v", err)
	}
	return doc
}

// A descoberta anuncia apenas os fluxos suportados (code + S256, sem implicit/
// ROPC) e o back-channel logout.
func TestDiscoveryHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	NewDiscoveryHandler(testDiscovery(t)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, quero 200", rec.Code)
	}
	var doc map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rts, _ := doc["response_types_supported"].([]any)
	if len(rts) != 1 || rts[0] != "code" {
		t.Fatalf("response_types_supported deveria ser só [code]: %v", rts)
	}
	cms, _ := doc["code_challenge_methods_supported"].([]any)
	if len(cms) != 1 || cms[0] != "S256" {
		t.Fatalf("code_challenge_methods deveria ser só [S256]: %v", cms)
	}
	if doc["backchannel_logout_supported"] != true {
		t.Fatalf("back-channel logout deveria ser anunciado")
	}
}

// O JWKS serve as chaves públicas que verificam os tokens.
func TestJWKSHandler(t *testing.T) {
	key, _ := oidc.GenerateSigningKey("kid-1")
	signer, _ := oidc.NewSigner(key)

	rec := httptest.NewRecorder()
	NewJWKSHandler(signer).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/jwks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, quero 200", rec.Code)
	}
	var set map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&set); err != nil {
		t.Fatalf("decode JWKS: %v", err)
	}
	keys, _ := set["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("o JWKS deveria ter 1 chave, veio %d", len(keys))
	}
}
