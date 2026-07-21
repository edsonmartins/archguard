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

package secretstore

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/domain"
)

func newStore(t *testing.T) *Provisional {
	t.Helper()
	ks, err := keystore.Open(filepath.Join(t.TempDir(), "secrets.sealed"), []byte("test-sealing-material"))
	if err != nil {
		t.Fatalf("keystore.Open: %v", err)
	}
	return NewProvisional(ks)
}

func TestSecretStoreRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	seed := []byte("totp-seed-\x00\x01\xff-binary")
	ref, err := s.Put(ctx, seed)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ref == "" {
		t.Fatal("ref vazia")
	}
	got, err := s.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, seed) {
		t.Errorf("segredo recuperado difere: %q != %q", got, seed)
	}
}

func TestSecretStoreDistinctRefs(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	r1, _ := s.Put(ctx, []byte("a"))
	r2, _ := s.Put(ctx, []byte("a"))
	if r1 == r2 {
		t.Error("Puts distintos deveriam gerar refs distintas")
	}
}

func TestSecretStoreNotFound(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get(context.Background(), "secret/inexistente"); !errors.Is(err, domain.ErrSecretNotFound) {
		t.Errorf("erro = %v, quer ErrSecretNotFound", err)
	}
}
