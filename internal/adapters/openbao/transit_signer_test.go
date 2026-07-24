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

package openbao

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeTransit mimics OpenBao's transit sign + read-key surface, and records whether
// any private-key path was ever requested (it must not be).
type fakeTransit struct {
	privateKeyRequested bool
}

func (f *fakeTransit) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/sign/"):
			var body struct {
				Input string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			// "Sign" by returning a deterministic function of the input — the app
			// never sees a private key.
			data, _ := base64.StdEncoding.DecodeString(body.Input)
			sig := base64.StdEncoding.EncodeToString(append([]byte("assinado:"), data...))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{"signature": "vault:v1:" + sig},
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/keys/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"latest_version": 1,
					"keys": map[string]any{
						"1": map[string]string{"public_key": "-----BEGIN PUBLIC KEY-----\nMII...\n-----END PUBLIC KEY-----\n"},
					},
				},
			})
		case strings.Contains(r.URL.Path, "/export/") || strings.Contains(r.URL.Path, "private"):
			// The transit engine exposes no private key; if the app ever asks, fail.
			f.privateKeyRequested = true
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newTransitSigner(t *testing.T) (*TransitSigner, *fakeTransit) {
	ft := &fakeTransit{}
	srv := ft.server(t)
	t.Cleanup(srv.Close)
	return NewTransitSigner(NewWithHTTP(srv.URL, "tok", srv.Client()), "transit"), ft
}

// Assinatura ocorre no cofre; a aplicação recebe a assinatura, nunca a chave
// privada (spec "Assinatura de selo").
func TestTransitSignerSign(t *testing.T) {
	signer, ft := newTransitSigner(t)
	sig, err := signer.Sign(context.Background(), "jwks-signing", []byte("cabecalho.payload"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.HasPrefix(string(sig), "assinado:") {
		t.Fatalf("assinatura inesperada: %q", sig)
	}
	if ft.privateKeyRequested {
		t.Fatalf("a aplicação JAMAIS deveria pedir a chave privada")
	}
}

// PublicKey devolve o PEM público (o único material que chega à aplicação).
func TestTransitSignerPublicKey(t *testing.T) {
	signer, _ := newTransitSigner(t)
	pem, err := signer.PublicKey(context.Background(), "jwks-signing")
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if !strings.Contains(string(pem), "BEGIN PUBLIC KEY") {
		t.Fatalf("PEM público inesperado: %q", pem)
	}
}

// Falha do cofre ao assinar propaga erro (fail-closed no caminho L3).
func TestTransitSignerFailClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	signer := NewTransitSigner(NewWithHTTP(srv.URL, "tok", srv.Client()), "transit")
	if _, err := signer.Sign(context.Background(), "k", []byte("x")); err == nil {
		t.Fatalf("cofre indisponível deveria falhar (fail-closed)")
	}
}
