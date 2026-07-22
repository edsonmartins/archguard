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
	"bytes"
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/adapters/wormanchor"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Exporta selos pendentes para o WORM, registra a âncora, e o objeto ancorado
// bate com o selo persistido; re-exportar é no-op.
func TestSealExporterExportPending(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	t.Cleanup(func() {
		bg := context.Background()
		conn, err := pool.Acquire(bg)
		if err == nil {
			defer conn.Release()
			_, _ = conn.Exec(bg, "SET session_replication_role = replica")
			_, _ = conn.Exec(bg, `DELETE FROM audit_seal_anchor WHERE seal_id IN
				(SELECT id FROM audit_seal WHERE organization_id = $1)`, org.String())
			_, _ = conn.Exec(bg, "SET session_replication_role = origin")
		}
	})

	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	w := NewAuditWriter(pool, fixedClock())
	sealer := NewAuditSealer(pool, signer, fixedClock())

	// Dois selos: [1,2] e [3,4].
	appendN(t, w, org, 2)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo 1: ok=%v err=%v", ok, err)
	}
	appendN(t, w, org, 2)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo 2: ok=%v err=%v", ok, err)
	}

	worm := wormanchor.NewMemory()
	exporter := NewSealExporter(pool, worm)

	n, err := exporter.ExportPending(ctx, "cliente-worm")
	if err != nil {
		t.Fatalf("ExportPending: %v", err)
	}
	if n != 2 {
		t.Fatalf("exportados = %d, quero 2", n)
	}

	// A âncora foi registrada para os dois selos.
	var anchored int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_seal_anchor a
		JOIN audit_seal s ON s.id = a.seal_id
		WHERE s.organization_id = $1 AND a.destination = 'cliente-worm'`, org.String()).Scan(&anchored); err != nil {
		t.Fatalf("count âncoras: %v", err)
	}
	if anchored != 2 {
		t.Fatalf("âncoras registradas = %d, quero 2", anchored)
	}

	// O objeto no WORM bate com o selo persistido (verificável offline).
	var ref string
	var seqEnd int64
	var headHash, sig []byte
	var keyID string
	if err := pool.QueryRow(ctx, `
		SELECT a.ref, s.seq_end, s.head_hash, s.key_id, s.signature
		FROM audit_seal_anchor a JOIN audit_seal s ON s.id = a.seal_id
		WHERE s.organization_id = $1 AND s.seq_end = 2`, org.String()).
		Scan(&ref, &seqEnd, &headHash, &keyID, &sig); err != nil {
		t.Fatalf("leitura selo/âncora: %v", err)
	}
	fetched, err := worm.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch WORM: %v", err)
	}
	if fetched.SeqEnd != seqEnd || !bytes.Equal(fetched.HeadHash, headHash) ||
		fetched.KeyID != keyID || !bytes.Equal(fetched.Signature, sig) {
		t.Fatalf("selo ancorado difere do persistido: %+v", fetched)
	}
	// E a assinatura ancorada verifica com a chave por key_id.
	content, err := domain.SealContent(org, fetched.SeqStart, fetched.SeqEnd, fetched.HeadHash, fetched.SealedAt)
	if err != nil {
		t.Fatalf("SealContent: %v", err)
	}
	if valid, err := signer.Verify(ctx, content, fetched.Signature, fetched.KeyID); err != nil || !valid {
		t.Fatalf("selo ancorado não verifica: valid=%v err=%v", valid, err)
	}

	// Re-exportar é no-op (idempotente).
	if n, err := exporter.ExportPending(ctx, "cliente-worm"); err != nil || n != 0 {
		t.Fatalf("re-exportação: n=%d err=%v, quero 0/nil", n, err)
	}

	// Um novo selo fica pendente e é exportado na próxima passada.
	appendN(t, w, org, 1)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo 3: ok=%v err=%v", ok, err)
	}
	if n, err := exporter.ExportPending(ctx, "cliente-worm"); err != nil || n != 1 {
		t.Fatalf("exportação do novo selo: n=%d err=%v, quero 1", n, err)
	}
}
