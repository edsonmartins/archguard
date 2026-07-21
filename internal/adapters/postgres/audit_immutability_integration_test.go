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
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Cenários "Tentativa de UPDATE" e "Tentativa de DELETE": a trigger de bloqueio
// (T-007) aborta a mutação NO BANCO — provado inclusive como SUPERUSUÁRIO (a
// trigger não é contornada por papel; a barreira de privilégio T-006 é a outra
// camada). Também cobre TRUNCATE.
func TestAuditEventImmutabilityTriggers(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)

	w := NewAuditWriter(pool, fixedClock())
	sealed, err := w.Append(ctx, minimalInput(org))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// UPDATE é rejeitado.
	_, err = pool.Exec(ctx,
		"UPDATE audit_event SET reason = 'adulterado' WHERE organization_id = $1 AND seq = $2",
		org.String(), sealed.Seq)
	if err == nil {
		t.Fatalf("UPDATE em audit_event deveria ser bloqueado pela trigger")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("erro de UPDATE não veio da trigger de bloqueio: %v", err)
	}

	// DELETE é rejeitado.
	_, err = pool.Exec(ctx,
		"DELETE FROM audit_event WHERE organization_id = $1 AND seq = $2", org.String(), sealed.Seq)
	if err == nil {
		t.Fatalf("DELETE em audit_event deveria ser bloqueado pela trigger")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("erro de DELETE não veio da trigger de bloqueio: %v", err)
	}

	// TRUNCATE é rejeitado.
	if _, err := pool.Exec(ctx, "TRUNCATE audit_event"); err == nil {
		t.Fatalf("TRUNCATE em audit_event deveria ser bloqueado pela trigger")
	}

	// O evento continua lá, íntegro.
	verifyChain(t, pool, org, 1)
}
