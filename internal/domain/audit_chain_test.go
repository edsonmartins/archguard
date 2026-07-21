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

package domain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Fixed vectors for the chain, pinned so a silent change to the hashing breaks
// the build (a drift invalidates every historical verification).
const (
	genesisGolden    = "ece65c55093858341da1b29df851f21c022025870a7d2031152cefeca46e8408"
	chainHash1Golden = "f30333f5a2c50dc074da36156d42345da0194a15e71047b7ee5c077f6c21b774"
)

func fixedNonce() []byte {
	n := make([]byte, AuditGenesisNonceSize)
	for i := range n {
		n[i] = byte(i)
	}
	return n
}

func TestGenesisHashFixedVector(t *testing.T) {
	org := uuid.MustParse("018f9a00-0000-7000-8000-0000000000ff")
	g, err := GenesisHash(org, fixedNonce())
	if err != nil {
		t.Fatalf("GenesisHash: %v", err)
	}
	if len(g) != AuditHashSize {
		t.Fatalf("genesis len = %d, quero %d", len(g), AuditHashSize)
	}
	if got := hex.EncodeToString(g); got != genesisGolden {
		t.Fatalf("genesis hash = %s, quero %s", got, genesisGolden)
	}

	// Nonce de tamanho errado é recusado (link de gênese malformado).
	if _, err := GenesisHash(org, []byte("curto")); !errors.Is(err, ErrInvalidGenesisNonce) {
		t.Fatalf("nonce curto: err = %v, quero ErrInvalidGenesisNonce", err)
	}
	// Nonce distinto ⇒ gênese distinta (dois tenants não compartilham cadeia).
	other := fixedNonce()
	other[0] ^= 0xFF
	g2, _ := GenesisHash(org, other)
	if bytes.Equal(g, g2) {
		t.Fatalf("gênese não deveria coincidir com nonce distinto")
	}
}

func TestSealEventChains(t *testing.T) {
	org := uuid.MustParse("018f9a00-0000-7000-8000-0000000000ff")
	genesis, err := GenesisHash(org, fixedNonce())
	if err != nil {
		t.Fatalf("GenesisHash: %v", err)
	}

	e1 := fixedEvent(t)
	s1, err := SealEvent(e1, genesis, 1)
	if err != nil {
		t.Fatalf("SealEvent 1: %v", err)
	}
	if s1.Seq != 1 || !bytes.Equal(s1.PrevHash, genesis) {
		t.Fatalf("primeiro elo errado: seq=%d", s1.Seq)
	}
	if len(s1.Hash) != AuditHashSize {
		t.Fatalf("hash len = %d", len(s1.Hash))
	}

	e2 := fixedEvent(t)
	e2.Action = ActionAuthLogout
	s2, err := SealEvent(e2, s1.Hash, 2)
	if err != nil {
		t.Fatalf("SealEvent 2: %v", err)
	}
	if !bytes.Equal(s2.PrevHash, s1.Hash) {
		t.Fatalf("segundo elo não aponta para o primeiro")
	}

	// Vetor fixo do primeiro hash da cadeia.
	if got := hex.EncodeToString(s1.Hash); got != chainHash1Golden {
		t.Fatalf("hash do elo 1 = %s, quero %s", got, chainHash1Golden)
	}

	// prev_hash malformado e seq inválido são recusados.
	if _, err := SealEvent(e1, []byte("curto"), 1); !errors.Is(err, ErrInvalidPrevHash) {
		t.Fatalf("prev curto: err = %v, quero ErrInvalidPrevHash", err)
	}
	if _, err := SealEvent(e1, genesis, 0); !errors.Is(err, ErrInvalidSeq) {
		t.Fatalf("seq 0: err = %v, quero ErrInvalidSeq", err)
	}
}

func TestVerifyLinkDetectsTampering(t *testing.T) {
	org := uuid.New()
	genesis, _ := GenesisHash(org, fixedNonce())
	s1, err := SealEvent(fixedEvent(t), genesis, 1)
	if err != nil {
		t.Fatalf("SealEvent: %v", err)
	}

	ok, err := VerifyLink(s1)
	if err != nil {
		t.Fatalf("VerifyLink: %v", err)
	}
	if !ok {
		t.Fatalf("elo íntegro deveria verificar")
	}

	// Conteúdo alterado ⇒ hash não recomputa.
	tampered := s1
	tampered.Event.Reason = "adulterado"
	if ok, _ := VerifyLink(tampered); ok {
		t.Fatalf("alteração de conteúdo deveria falhar a verificação do elo")
	}

	// prev_hash alterado (reordenação/remoção altera o elo) ⇒ hash não bate.
	tampered2 := s1
	tampered2.PrevHash = make([]byte, AuditHashSize) // zeros
	if ok, _ := VerifyLink(tampered2); ok {
		t.Fatalf("alteração de prev_hash deveria falhar a verificação do elo")
	}
}
