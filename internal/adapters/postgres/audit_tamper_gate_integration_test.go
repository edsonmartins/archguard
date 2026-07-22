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
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTamperGate is the package gate for tamper detection (RFC-0003 §6): it
// seeds a sealed chain, then applies each tampering class DIRECTLY in the
// database (bypassing the append-only triggers as a superuser could) and
// confirms the verifier — anchored on the REAL signer, so seal signatures are
// checked too — catches each one at the right seq. It covers the three classes
// the spec names (alteração, remoção, reordenação) plus an invalid seal.
func TestTamperGate(t *testing.T) {
	cases := []struct {
		name     string
		tamper   func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID)
		wantKind domain.DivergenceKind
		wantSeq  int64
	}{
		{
			name: "alteração de conteúdo",
			tamper: func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
				bypassExec(t, pool, "UPDATE audit_event SET reason = 'adulterado' WHERE organization_id = $1 AND seq = 3", org.String())
			},
			wantKind: domain.DivergenceAltered, wantSeq: 3,
		},
		{
			name: "remoção de evento",
			tamper: func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
				bypassExec(t, pool, "DELETE FROM audit_event WHERE organization_id = $1 AND seq = 3", org.String())
			},
			wantKind: domain.DivergenceRemoved, wantSeq: 3,
		},
		{
			name: "reordenação (troca de conteúdo entre eventos)",
			tamper: func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
				// Troca o reason dos eventos 2 e 4 — reordenar conteúdo quebra o
				// hash já no primeiro evento afetado.
				bypassExec(t, pool, "UPDATE audit_event SET reason = 'do-4' WHERE organization_id = $1 AND seq = 2", org.String())
				bypassExec(t, pool, "UPDATE audit_event SET reason = 'do-2' WHERE organization_id = $1 AND seq = 4", org.String())
			},
			wantKind: domain.DivergenceAltered, wantSeq: 2,
		},
		{
			name: "quebra de cadeia (prev_hash)",
			tamper: func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
				bypassExec(t, pool, "UPDATE audit_event SET prev_hash = decode(repeat('00',32),'hex') WHERE organization_id = $1 AND seq = 4", org.String())
			},
			wantKind: domain.DivergenceBrokenChain, wantSeq: 4,
		},
		{
			name: "selo inválido",
			tamper: func(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
				// Adultera o head_hash do selo → a assinatura não confere mais.
				bypassExec(t, pool, "UPDATE audit_seal SET head_hash = decode('00','hex') || head_hash WHERE organization_id = $1", org.String())
			},
			wantKind: domain.DivergenceSealInvalid, wantSeq: 0, // qualquer seq_end do selo
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := setupTenantPool(t)
			ctx := context.Background()
			org := uuid.New()
			cleanupAudit(t, pool, org)

			signer, err := auditseal.NewProvisional()
			if err != nil {
				t.Fatalf("NewProvisional: %v", err)
			}
			w := NewAuditWriter(pool, fixedClock())
			appendN(t, w, org, 5)
			if _, ok, err := NewAuditSealer(pool, signer, fixedClock()).SealOrganization(ctx, org); err != nil || !ok {
				t.Fatalf("selo: ok=%v err=%v", ok, err)
			}

			// Verificador ancorado no MESMO assinante (assinaturas conferíveis).
			verifier := NewAuditVerifier(pool, signer)

			// Baseline: íntegra e selada verifica.
			if rep, err := verifier.VerifyOrganization(ctx, org); err != nil || !rep.OK || !rep.SealSignaturesChecked {
				t.Fatalf("baseline deveria verificar com assinaturas: rep=%+v err=%v", rep, err)
			}

			// Adultera e confirma a detecção.
			tc.tamper(t, pool, org)
			rep, err := verifier.VerifyOrganization(ctx, org)
			if err != nil {
				t.Fatalf("VerifyOrganization: %v", err)
			}
			if rep.OK {
				t.Fatalf("%s: adulteração NÃO detectada", tc.name)
			}
			if rep.Kind != tc.wantKind {
				t.Fatalf("%s: tipo = %q, quero %q (detalhe: %s)", tc.name, rep.Kind, tc.wantKind, rep.Detail)
			}
			if tc.wantSeq != 0 && rep.FirstDivergence != tc.wantSeq {
				t.Fatalf("%s: primeiro seq = %d, quero %d", tc.name, rep.FirstDivergence, tc.wantSeq)
			}
		})
	}
}
