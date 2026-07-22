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

package postgres

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// A exportação é AUTO-VERIFICÁVEL: com nada além do NDJSON, um auditor recompõe
// a cadeia (com a gênese exportada) e confere as assinaturas dos selos (com as
// chaves públicas exportadas) — offline.
func TestTrailExportSelfVerifiable(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)

	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	w := NewAuditWriter(pool, fixedClock())
	sealer := NewAuditSealer(pool, signer, fixedClock())
	appendN(t, w, org, 3)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo: ok=%v err=%v", ok, err)
	}

	// Exporta.
	resolve := func(keyID string) ([]byte, bool) {
		pub := signer.PublicKey(keyID)
		return pub, pub != nil
	}
	var buf bytes.Buffer
	if err := NewTrailExporter(pool, nil).Export(ctx, org, resolve, fixedClock(), &buf); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// --- daqui para baixo, verificação OFFLINE só com os bytes exportados ---
	var (
		meta       exportMeta
		pubKeys    = map[string]ed25519.PublicKey{}
		events     []domain.SealedEvent
		seals      []exportSeal
		hasProcess bool
	)
	sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			t.Fatalf("linha NDJSON inválida: %v", err)
		}
		switch peek.Type {
		case "meta":
			if err := json.Unmarshal(line, &meta); err != nil {
				t.Fatalf("meta: %v", err)
			}
		case "public_key":
			var pk exportPublicKey
			if err := json.Unmarshal(line, &pk); err != nil {
				t.Fatalf("public_key: %v", err)
			}
			raw, err := hex.DecodeString(pk.PublicKeyHex)
			if err != nil {
				t.Fatalf("hex pubkey: %v", err)
			}
			pubKeys[pk.KeyID] = ed25519.PublicKey(raw)
		case "event":
			var ev exportEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				t.Fatalf("event: %v", err)
			}
			prev, _ := hex.DecodeString(ev.PrevHashHex)
			h, _ := hex.DecodeString(ev.HashHex)
			events = append(events, domain.SealedEvent{Event: ev.Event, Seq: ev.Seq, PrevHash: prev, Hash: h})
		case "seal":
			var s exportSeal
			if err := json.Unmarshal(line, &s); err != nil {
				t.Fatalf("seal: %v", err)
			}
			seals = append(seals, s)
		case "procedure":
			hasProcess = true
		default:
			t.Fatalf("tipo de registro desconhecido: %q", peek.Type)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Estrutura mínima presente.
	if !meta.HasChain || meta.OrganizationID != org.String() || len(events) != 3 || len(seals) != 1 || !hasProcess {
		t.Fatalf("export incompleto: meta=%+v eventos=%d selos=%d proc=%v", meta, len(events), len(seals), hasProcess)
	}

	// 1) Recompõe a cadeia a partir da gênese exportada.
	genesis, err := hex.DecodeString(meta.GenesisHashHex)
	if err != nil {
		t.Fatalf("hex gênese: %v", err)
	}
	if rep := domain.VerifyChain(genesis, events); !rep.OK {
		t.Fatalf("cadeia exportada não verifica: %+v", rep)
	}

	// 2) Confere a assinatura do selo com a chave pública exportada (offline).
	for _, s := range seals {
		pub, ok := pubKeys[s.KeyID]
		if !ok {
			t.Fatalf("chave pública do selo %s ausente no export", s.KeyID)
		}
		headHash, _ := hex.DecodeString(s.HeadHashHex)
		sig, _ := hex.DecodeString(s.SignatureHex)
		content, err := domain.SealContent(org, s.SeqStart, s.SeqEnd, headHash, s.SealedAtUS)
		if err != nil {
			t.Fatalf("SealContent: %v", err)
		}
		if !ed25519.Verify(pub, content, sig) {
			t.Fatalf("assinatura do selo exportado não confere (offline)")
		}
		// E o head do selo é o hash do último evento do intervalo.
		if !bytes.Equal(headHash, events[s.SeqEnd-1].Hash) {
			t.Fatalf("head_hash do selo não bate com o evento em seq %d", s.SeqEnd)
		}
	}
}
