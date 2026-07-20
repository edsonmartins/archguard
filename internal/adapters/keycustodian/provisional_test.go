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

package keycustodian

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

func key(t *testing.T, b byte) []byte {
	t.Helper()
	k := make([]byte, minDeploymentKeyBytes)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestNewProvisionalRejectsWeakKey(t *testing.T) {
	if _, err := NewProvisional(make([]byte, minDeploymentKeyBytes-1)); !errors.Is(err, ErrWeakDeploymentKey) {
		t.Errorf("chave curta deveria ser rejeitada, erro = %v", err)
	}
	if _, err := NewProvisional(nil); !errors.Is(err, ErrWeakDeploymentKey) {
		t.Errorf("chave nil deveria ser rejeitada, erro = %v", err)
	}
	if _, err := NewProvisional(key(t, 0x01)); err != nil {
		t.Errorf("chave de 256 bits deveria ser aceita, erro = %v", err)
	}
}

func TestHashEmailIsDeterministicAndCorrect(t *testing.T) {
	k := key(t, 0x2b)
	c, err := NewProvisional(k)
	if err != nil {
		t.Fatal(err)
	}
	h1, err := c.HashEmail("Alice@Empresa.com")
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	// Determinístico e case/space-insensitive (via normalização).
	h2, err := c.HashEmail("  alice@empresa.COM ")
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	if !bytes.Equal(h1, h2) {
		t.Error("mesmo e-mail (após normalização) deveria dar o mesmo hash")
	}
	// Confere contra o HMAC-SHA256 esperado sobre a forma normalizada.
	mac := hmac.New(sha256.New, k)
	mac.Write([]byte("alice@empresa.com"))
	if !bytes.Equal(h1, mac.Sum(nil)) {
		t.Error("hash não corresponde a HMAC-SHA256(chave, normalizado)")
	}
	if len(h1) != sha256.Size {
		t.Errorf("hash tem %d bytes, quer %d", len(h1), sha256.Size)
	}
}

func TestHashEmailDistinctInputsAndKeys(t *testing.T) {
	c, _ := NewProvisional(key(t, 0x2b))
	a, _ := c.HashEmail("alice@x.com")
	b, _ := c.HashEmail("bob@x.com")
	if bytes.Equal(a, b) {
		t.Error("e-mails distintos deveriam dar hashes distintos")
	}
	// Chave de deployment distinta ⇒ hash distinto para o mesmo e-mail.
	other, _ := NewProvisional(key(t, 0x99))
	a2, _ := other.HashEmail("alice@x.com")
	if bytes.Equal(a, a2) {
		t.Error("chaves de deployment distintas deveriam dar hashes distintos")
	}
}

func TestHashEmailRejectsEmpty(t *testing.T) {
	c, _ := NewProvisional(key(t, 0x2b))
	for _, in := range []string{"", "   ", "\t\n"} {
		if _, err := c.HashEmail(in); !errors.Is(err, domain.ErrEmptyEmail) {
			t.Errorf("HashEmail(%q) erro = %v, quer ErrEmptyEmail", in, err)
		}
	}
}

func TestNewProvisionalCopiesKey(t *testing.T) {
	k := key(t, 0x2b)
	c, _ := NewProvisional(k)
	h1, _ := c.HashEmail("alice@x.com")
	// Mutar a fatia original NÃO deve afetar o custodiante (cópia defensiva).
	for i := range k {
		k[i] = 0x00
	}
	h2, _ := c.HashEmail("alice@x.com")
	if !bytes.Equal(h1, h2) {
		t.Error("mutação da chave original afetou o custodiante — faltou cópia defensiva")
	}
}
