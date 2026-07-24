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

// Um lote com e-mail duplicado NÃO é fundido automaticamente: os registros em
// conflito são reportados (ImportConflicted) e nenhuma identidade é criada para
// eles — a fusão assistida (identfusion, com aprovação humana) resolve depois.
func TestImportConflictNotAutoMerged(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	org := makeTenant(t, pool, "import-conflict")
	importer := NewIdentityImporter(pool, cust)

	dupHash, _ := cust.HashEmail("dup@cli.com")
	uniqHash, _ := cust.HashEmail("uniq@cli.com")
	t.Cleanup(func() {
		for _, h := range [][]byte{dupHash, uniqHash} {
			_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", h)
			_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", h)
		}
	})

	report, err := importer.Import(ctx, org.orgID, "lote-conflito", []domain.ImportRecord{
		{Email: "dup@cli.com", DisplayName: "Registro A"},
		{Email: "DUP@cli.com", DisplayName: "Registro B"}, // mesma pessoa, divergente
		{Email: "uniq@cli.com", DisplayName: "Único"},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(report.Conflicts) != 1 {
		t.Fatalf("esperava 1 conflito reportado, veio %d", len(report.Conflicts))
	}
	if report.Count(domain.ImportConflicted) != 2 {
		t.Fatalf("os 2 registros em conflito deveriam ser ImportConflicted, veio %d", report.Count(domain.ImportConflicted))
	}
	if report.Count(domain.ImportCreated) != 1 {
		t.Fatalf("o único deveria ser criado, veio %d", report.Count(domain.ImportCreated))
	}

	// NENHUMA identidade foi criada para o e-mail em conflito (sem fusão silenciosa).
	var dupCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", dupHash).Scan(&dupCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if dupCount != 0 {
		t.Fatalf("e-mail em conflito NÃO deveria ter identidade importada (sem auto-merge), veio %d", dupCount)
	}
	// O único foi criado.
	var uniqCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", uniqHash).Scan(&uniqCount); err != nil {
		t.Fatalf("count uniq: %v", err)
	}
	if uniqCount != 1 {
		t.Fatalf("o único deveria ter sido importado, veio %d", uniqCount)
	}
}
