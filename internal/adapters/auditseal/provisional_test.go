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

package auditseal

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

func sealContent(t *testing.T) []byte {
	t.Helper()
	head := make([]byte, domain.AuditHashSize)
	for i := range head {
		head[i] = byte(i)
	}
	c, err := domain.SealContent(uuid.New(), 1, 100, head, 1_700_000_000_000_000)
	if err != nil {
		t.Fatalf("SealContent: %v", err)
	}
	return c
}

func TestProvisionalSignVerifyRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	content := sealContent(t)

	sig, keyID, err := s.Sign(ctx, content)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	ok, err := s.Verify(ctx, content, sig, keyID)
	if err != nil || !ok {
		t.Fatalf("Verify válido: ok=%v err=%v", ok, err)
	}

	// Conteúdo adulterado ⇒ assinatura não confere.
	tampered := append([]byte{}, content...)
	tampered[0] ^= 0xFF
	if ok, _ := s.Verify(ctx, tampered, sig, keyID); ok {
		t.Fatalf("conteúdo adulterado não deveria verificar")
	}

	// key_id desconhecido ⇒ fail-closed.
	if _, err := s.Verify(ctx, content, sig, "chave-inexistente"); !errors.Is(err, domain.ErrSealKeyUnknown) {
		t.Fatalf("key_id desconhecido: err = %v, quero ErrSealKeyUnknown", err)
	}
}

// Verificação histórica após rotação (RFC-0003 §4): um selo assinado com a chave
// antiga continua verificável pelo seu key_id depois que a chave é rotacionada.
func TestProvisionalVerifiesAfterRotation(t *testing.T) {
	ctx := context.Background()
	s, err := NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	content := sealContent(t)

	sigOld, keyOld, err := s.Sign(ctx, content)
	if err != nil {
		t.Fatalf("Sign antigo: %v", err)
	}

	keyNew, err := s.Rotate()
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if keyNew == keyOld {
		t.Fatalf("rotação deveria produzir novo key_id")
	}

	// O selo antigo verifica com o key_id antigo.
	if ok, err := s.Verify(ctx, content, sigOld, keyOld); err != nil || !ok {
		t.Fatalf("selo antigo pós-rotação: ok=%v err=%v", ok, err)
	}
	// Uma nova assinatura usa a chave corrente (novo key_id).
	sigNew, keyID, err := s.Sign(ctx, content)
	if err != nil {
		t.Fatalf("Sign novo: %v", err)
	}
	if keyID != keyNew {
		t.Fatalf("assinatura nova deveria usar a chave corrente, veio %s", keyID)
	}
	// A assinatura antiga NÃO verifica sob o key_id novo (chaves distintas).
	if ok, _ := s.Verify(ctx, content, sigOld, keyNew); ok {
		t.Fatalf("assinatura antiga não deveria verificar com a chave nova")
	}
	if ok, err := s.Verify(ctx, content, sigNew, keyNew); err != nil || !ok {
		t.Fatalf("selo novo: ok=%v err=%v", ok, err)
	}

	if pub := s.PublicKey(keyOld); pub == nil || bytes.Equal(pub, s.PublicKey(keyNew)) {
		t.Fatalf("chaves públicas deveriam ser distintas e disponíveis por key_id")
	}
}

func TestSealContentRejectsInvalid(t *testing.T) {
	head := make([]byte, domain.AuditHashSize)
	if _, err := domain.SealContent(uuid.Nil, 1, 10, head, 0); !errors.Is(err, domain.ErrInvalidSeal) {
		t.Fatalf("org nula: err = %v, quero ErrInvalidSeal", err)
	}
	if _, err := domain.SealContent(uuid.New(), 10, 1, head, 0); !errors.Is(err, domain.ErrInvalidSeal) {
		t.Fatalf("intervalo invertido: err = %v, quero ErrInvalidSeal", err)
	}
	if _, err := domain.SealContent(uuid.New(), 1, 10, []byte("curto"), 0); !errors.Is(err, domain.ErrInvalidSeal) {
		t.Fatalf("head curto: err = %v, quero ErrInvalidSeal", err)
	}
}
