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
	"testing"

	"github.com/google/uuid"
)

// buildChain seals n events off a genesis, returning the genesis and the sealed
// chain — the fixture the verifier walks.
func buildChain(t *testing.T, n int) ([]byte, []SealedEvent) {
	t.Helper()
	org := uuid.New()
	genesis, err := GenesisHash(org, fixedNonce())
	if err != nil {
		t.Fatalf("GenesisHash: %v", err)
	}
	prev := genesis
	var chain []SealedEvent
	for i := 0; i < n; i++ {
		ev := fixedEvent(t)
		ev.OrganizationID = org
		ev.Reason = "evento " + string(rune('a'+i))
		s, err := SealEvent(ev, prev, int64(i+1))
		if err != nil {
			t.Fatalf("SealEvent %d: %v", i, err)
		}
		chain = append(chain, s)
		prev = s.Hash
	}
	return genesis, chain
}

func TestVerifyChainIntact(t *testing.T) {
	genesis, chain := buildChain(t, 5)
	rep := VerifyChain(genesis, chain)
	if !rep.OK || rep.EventsChecked != 5 {
		t.Fatalf("cadeia íntegra deveria verificar: %+v", rep)
	}
}

// Alteração: mudar o conteúdo de um evento sem recomputar o hash ⇒ detectado
// como altered no seq afetado.
func TestVerifyChainDetectsAlteration(t *testing.T) {
	genesis, chain := buildChain(t, 5)
	chain[2].Event.Reason = "conteúdo adulterado" // seq 3, hash não recomputa
	rep := VerifyChain(genesis, chain)
	if rep.OK || rep.Kind != DivergenceAltered || rep.FirstDivergence != 3 {
		t.Fatalf("alteração deveria ser detectada no seq 3: %+v", rep)
	}
}

// Remoção: apagar um evento do meio ⇒ lacuna de seq detectada.
func TestVerifyChainDetectsRemoval(t *testing.T) {
	genesis, chain := buildChain(t, 5)
	withGap := append([]SealedEvent{}, chain[:2]...) // seq 1,2
	withGap = append(withGap, chain[3:]...)          // seq 4,5 (removido o 3)
	rep := VerifyChain(genesis, withGap)
	if rep.OK || rep.Kind != DivergenceRemoved || rep.FirstDivergence != 3 {
		t.Fatalf("remoção deveria ser detectada no seq 3: %+v", rep)
	}
}

// Reordenação / quebra de cadeia: trocar prev_hash de um elo ⇒ broken_chain.
func TestVerifyChainDetectsBrokenChain(t *testing.T) {
	genesis, chain := buildChain(t, 5)
	chain[3].PrevHash = make([]byte, AuditHashSize) // seq 4: prev_hash zerado
	rep := VerifyChain(genesis, chain)
	if rep.OK || rep.Kind != DivergenceBrokenChain || rep.FirstDivergence != 4 {
		t.Fatalf("quebra de cadeia deveria ser detectada no seq 4: %+v", rep)
	}
}

// Gênese trocada ⇒ o primeiro elo não encadeia.
func TestVerifyChainDetectsGenesisMismatch(t *testing.T) {
	_, chain := buildChain(t, 3)
	wrongGenesis := make([]byte, AuditHashSize)
	rep := VerifyChain(wrongGenesis, chain)
	if rep.OK || rep.Kind != DivergenceBrokenChain || rep.FirstDivergence != 1 {
		t.Fatalf("gênese trocada deveria quebrar no seq 1: %+v", rep)
	}
}

func TestParseOutcomeRoundTrip(t *testing.T) {
	for _, o := range []Outcome{Allowed, Denied, Failed} {
		s := AuditEvent{Outcome: o}.SerializedOutcome()
		got, err := ParseOutcome(s)
		if err != nil || got != o {
			t.Fatalf("round-trip de outcome %v: got=%v err=%v", o, got, err)
		}
	}
	if _, err := ParseOutcome("nonsense"); err == nil {
		t.Fatalf("outcome inválido deveria falhar")
	}
}
