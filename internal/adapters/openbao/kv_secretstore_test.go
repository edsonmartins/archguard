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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

// fakeVault mimics OpenBao's KV v2 HTTP surface for the store test, and checks the
// auth token is presented.
type fakeVault struct {
	mu    sync.Mutex
	data  map[string]string // path -> base64 value
	token string
}

func newFakeVault(token string) *fakeVault {
	return &fakeVault{data: map[string]string{}, token: token}
}

func (f *fakeVault) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != f.token {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()

		switch {
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/data/"):
			var body struct {
				Data struct {
					Value string `json:"value"`
				} `json:"data"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.data[pathKey(r.URL.Path, "/data/")] = body.Data.Value
			w.WriteHeader(http.StatusOK)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/data/"):
			v, ok := f.data[pathKey(r.URL.Path, "/data/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp := map[string]any{"data": map[string]any{"data": map[string]string{"value": v}}}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == "DELETE" && strings.Contains(r.URL.Path, "/metadata/"):
			delete(f.data, pathKey(r.URL.Path, "/metadata/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func pathKey(urlPath, seg string) string {
	i := strings.Index(urlPath, seg)
	return urlPath[i+len(seg):]
}

func newStore(t *testing.T) (*KVSecretStore, *fakeVault) {
	fv := newFakeVault("token-secreto")
	srv := fv.server(t)
	t.Cleanup(srv.Close)
	return NewKVSecretStore(NewWithHTTP(srv.URL, "token-secreto", srv.Client()), "secret"), fv
}

// Put→Get round-trip: o segredo é custodiado no cofre e resolvido pela referência.
func TestKVSecretStoreRoundTrip(t *testing.T) {
	store, fv := newStore(t)
	ctx := context.Background()
	secret := []byte("client-secret-super-sigiloso")

	ref, err := store.Put(ctx, secret)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !strings.HasPrefix(ref, "openbao:kv:") {
		t.Fatalf("referência inesperada: %q", ref)
	}
	// O segredo em claro NÃO aparece na referência (só o path opaco).
	if bytes.Contains([]byte(ref), secret) {
		t.Fatalf("a referência não deveria conter o segredo")
	}
	// O cofre guarda o valor (base64), não em claro na referência.
	if len(fv.data) != 1 {
		t.Fatalf("o cofre deveria ter 1 segredo, tem %d", len(fv.data))
	}

	got, err := store.Get(ctx, ref)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("Get inesperado: %q err=%v", got, err)
	}
}

// Referência ausente → domain.ErrSecretNotFound.
func TestKVSecretStoreNotFound(t *testing.T) {
	store, _ := newStore(t)
	if _, err := store.Get(context.Background(), "openbao:kv:inexistente"); err != domain.ErrSecretNotFound {
		t.Fatalf("esperava ErrSecretNotFound, veio %v", err)
	}
}

// Delete remove do cofre e é idempotente (compensação).
func TestKVSecretStoreDelete(t *testing.T) {
	store, fv := newStore(t)
	ctx := context.Background()
	ref, _ := store.Put(ctx, []byte("x"))

	if err := store.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(fv.data) != 0 {
		t.Fatalf("o segredo deveria ter sido removido do cofre")
	}
	// Idempotente.
	if err := store.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete idempotente falhou: %v", err)
	}
	if _, err := store.Get(ctx, ref); err != domain.ErrSecretNotFound {
		t.Fatalf("após delete deveria ser ErrSecretNotFound, veio %v", err)
	}
}

// Token errado → o cofre recusa; o store propaga erro (fail-closed).
func TestKVSecretStoreWrongToken(t *testing.T) {
	fv := newFakeVault("token-certo")
	srv := fv.server(t)
	t.Cleanup(srv.Close)
	store := NewKVSecretStore(NewWithHTTP(srv.URL, "token-ERRADO", srv.Client()), "secret")
	if _, err := store.Put(context.Background(), []byte("x")); err == nil {
		t.Fatalf("token errado deveria falhar")
	}
}
