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
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

func newCipher(t *testing.T) *Provisional {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 3)
	}
	p, err := NewProvisional(key)
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	return p
}

// Round-trip por titular: cifra e decifra o campo pessoal.
func TestSubjectCipherRoundTrip(t *testing.T) {
	p := newCipher(t)
	plain := []byte("ana@cli.com")
	ct, err := p.EncryptForSubject("subj-1", plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatalf("o ciphertext NÃO deveria conter o texto claro")
	}
	got, err := p.DecryptForSubject("subj-1", ct)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Decrypt inesperado: %q err=%v", got, err)
	}
}

// Crypto-shredding: destruir a chave torna o dado IRRECUPERÁVEL (ADR-0014).
func TestSubjectCipherCryptoShredding(t *testing.T) {
	p := newCipher(t)
	ct, _ := p.EncryptForSubject("titular", []byte("dado pessoal"))

	if err := p.DestroySubjectKey("titular"); err != nil {
		t.Fatalf("DestroySubjectKey: %v", err)
	}
	// Decifrar o MESMO ciphertext após o shredding falha — dado perdido de vez.
	if _, err := p.DecryptForSubject("titular", ct); !errors.Is(err, domain.ErrSubjectKeyDestroyed) {
		t.Fatalf("após shredding a decifragem deveria ser ErrSubjectKeyDestroyed, veio %v", err)
	}
	// Não se escreve novo dado pessoal para um titular eliminado.
	if _, err := p.EncryptForSubject("titular", []byte("novo")); !errors.Is(err, domain.ErrSubjectKeyDestroyed) {
		t.Fatalf("cifrar para titular eliminado deveria falhar, veio %v", err)
	}
	// Idempotente.
	if err := p.DestroySubjectKey("titular"); err != nil {
		t.Fatalf("destruir de novo deveria ser no-op: %v", err)
	}
}

// Isolamento entre titulares: a chave de um não decifra o dado de outro.
func TestSubjectCipherIsolation(t *testing.T) {
	p := newCipher(t)
	ct, _ := p.EncryptForSubject("a", []byte("segredo de A"))
	if _, err := p.DecryptForSubject("b", ct); err == nil {
		t.Fatalf("a chave de B não deveria decifrar o dado de A")
	}
	// Destruir A não afeta B.
	_, _ = p.EncryptForSubject("b", []byte("segredo de B"))
	_ = p.DestroySubjectKey("a")
	if _, err := p.EncryptForSubject("b", []byte("mais de B")); err != nil {
		t.Fatalf("destruir A não deveria afetar B: %v", err)
	}
}

// Decifrar para titular nunca cifrado -> ErrSubjectKeyMissing.
func TestSubjectCipherMissing(t *testing.T) {
	p := newCipher(t)
	if _, err := p.DecryptForSubject("nunca", []byte("x")); !errors.Is(err, domain.ErrSubjectKeyMissing) {
		t.Fatalf("titular sem chave deveria ser ErrSubjectKeyMissing, veio %v", err)
	}
}
