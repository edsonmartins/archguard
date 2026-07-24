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

	"github.com/casdoor/casdoor/internal/domain"
)

// Lote importado: identidades entram SEM credencial (enrolamento obrigatório no
// primeiro acesso — nenhuma senha da origem é aceita, RFC-0007 §4); dedup por
// email_hash reusa a identidade conhecida (só membership), nunca duplica.
func TestIdentityImporterBatch(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	org := makeTenant(t, pool, "import")
	importer := NewIdentityImporter(pool, cust)

	// Pré-existente (será "reused").
	knownHash, _ := cust.HashEmail("ja@existe.com")
	if _, err := NewDirectoryProvisioner(pool, cust).ProvisionUser(ctx, org.orgID,
		domain.DirectorySyncRecord{Email: "ja@existe.com", Active: true}); err != nil {
		t.Fatalf("pré-provisão: %v", err)
	}

	novoHash, _ := cust.HashEmail("nova@pessoa.com")
	t.Cleanup(func() {
		for _, h := range [][]byte{knownHash, novoHash} {
			_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", h)
			_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", h)
		}
	})

	report, err := importer.Import(ctx, org.orgID, "lote-2026-07-24", []domain.ImportRecord{
		{Email: "nova@pessoa.com", DisplayName: "Nova"},
		{Email: "ja@existe.com", DisplayName: "Já"},
		{DisplayName: "sem email"}, // inválido -> failed
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if report.Count(domain.ImportCreated) != 1 {
		t.Fatalf("esperava 1 criado, veio %d", report.Count(domain.ImportCreated))
	}
	if report.Count(domain.ImportReused) != 1 {
		t.Fatalf("esperava 1 reusado (dedup), veio %d", report.Count(domain.ImportReused))
	}
	if report.Count(domain.ImportFailed) != 1 {
		t.Fatalf("esperava 1 falho (sem e-mail), veio %d", report.Count(domain.ImportFailed))
	}
	if report.BatchID != "lote-2026-07-24" {
		t.Fatalf("batch id não preservado")
	}

	// A identidade importada NÃO tem credencial — enrolamento obrigatório no login.
	var credCount int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM credential WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)",
		novoHash).Scan(&credCount); err != nil {
		t.Fatalf("count credenciais: %v", err)
	}
	if credCount != 0 {
		t.Fatalf("identidade importada NÃO deveria ter credencial (nenhuma senha importada), veio %d", credCount)
	}

	// Dedup: exatamente uma identidade para o e-mail conhecido.
	var identCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", knownHash).Scan(&identCount); err != nil {
		t.Fatalf("count identidades: %v", err)
	}
	if identCount != 1 {
		t.Fatalf("import NÃO deveria duplicar identidade conhecida, veio %d", identCount)
	}
}
