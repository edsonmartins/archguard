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

package openbao_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/openbao"
	"github.com/casdoor/casdoor/internal/domain"
)

func TestTransitCustodianHashEmail(t *testing.T) {
	raw := []byte("0123456789abcdef0123456789abcdef") // 32-byte HMAC
	b64 := base64.StdEncoding.EncodeToString(raw)

	var gotPath, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotToken = r.URL.Path, r.Header.Get("X-Vault-Token")
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/transit/hmac/") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":{"hmac":"vault:v1:%s"}}`, b64)
	}))
	defer srv.Close()

	c := openbao.NewTransitCustodian(openbao.NewWithHTTP(srv.URL, "tok-123", srv.Client()), "transit", "archguard-email-hash")

	got, err := c.HashEmail("Admin@Example.com")
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("hash = %x, want %x", got, raw)
	}
	if gotPath != "/v1/transit/hmac/archguard-email-hash" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotToken != "tok-123" {
		t.Fatalf("token não enviado: %q", gotToken)
	}
}

func TestTransitCustodianEmptyEmailFailsClosed(t *testing.T) {
	// No server call should happen; an empty e-mail is refused up front.
	c := openbao.NewTransitCustodian(openbao.New("http://unused", "t"), "transit", "k")
	if _, err := c.HashEmail("   "); !errors.Is(err, domain.ErrEmptyEmail) {
		t.Fatalf("empty email: want ErrEmptyEmail, got %v", err)
	}
}

func TestTransitCustodianVaultErrorFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // vault down
	}))
	defer srv.Close()
	c := openbao.NewTransitCustodian(openbao.NewWithHTTP(srv.URL, "t", srv.Client()), "transit", "k")
	if _, err := c.HashEmail("admin@example.com"); err == nil {
		t.Fatalf("a vault failure must be an error (fail-closed), got nil")
	}
}
