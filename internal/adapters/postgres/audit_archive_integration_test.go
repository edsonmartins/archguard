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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueSuffix is a DDL-safe unique identifier fragment (hex only).
func uniqueSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

// dropArchivedTable drops a detached/attached partition table at test end.
func dropArchivedTable(t *testing.T, pool *pgxpool.Pool, name string) {
	// Best-effort: if still attached, detach first, then drop.
	bg := context.Background()
	_, _ = pool.Exec(bg, "ALTER TABLE audit_event DETACH PARTITION "+name)
	_, _ = pool.Exec(bg, "DROP TABLE IF EXISTS "+name)
}

// countRows runs a scalar count query.
func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("countRows: %v", err)
	}
	return n
}

// Arquivamento: uma partição de tempo SELADA é destacada (eventos preservados,
// nada deletado) e depois restaurada; a cadeia volta íntegra. Não-selada é
// recusada.
func TestPartitionArchiveRestore(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := adminCtx()
	org := uuid.New()
	cleanupAudit(t, pool, org)

	// Uma janela de tempo dedicada e uma partição para ela.
	from := time.Date(2035, 3, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	partition := "audit_event_2035_03_" + uniqueSuffix()
	if err := EnsureTimePartition(ctx, pool, partition, from, to); err != nil {
		t.Fatalf("EnsureTimePartition: %v", err)
	}
	t.Cleanup(func() { dropArchivedTable(t, pool, partition) })

	// Escreve 3 eventos com occurred_at DENTRO da janela (roteiam para a partição).
	clockIn := func() time.Time { return from.Add(24 * time.Hour) }
	w := NewAuditWriter(pool, clockIn)
	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	appendN(t, w, org, 3)

	// O emissor do arquivamento carimba com "agora" (FORA da janela → partição
	// DEFAULT), senão o próprio evento de arquivo cairia na partição arquivada.
	clockNow := func() time.Time { return to.Add(15 * 24 * time.Hour) }
	archiver := NewPartitionArchiver(pool, NewAuditWriter(pool, clockNow))

	// Ainda NÃO selada: arquivamento recusado.
	if err := archiver.Archive(ctx, partition, from, to); !errors.Is(err, ErrPartitionUnsealed) {
		t.Fatalf("período não selado: err = %v, quero ErrPartitionUnsealed", err)
	}

	// Sela e arquiva.
	if _, ok, err := NewAuditSealer(pool, signer, clockIn).SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo: ok=%v err=%v", ok, err)
	}
	if err := archiver.Archive(ctx, partition, from, to); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	// Os eventos da janela saíram da tabela particionada MAS estão preservados na
	// tabela destacada (nada deletado).
	if n := countRows(t, pool, `SELECT count(*) FROM audit_event WHERE occurred_at >= $1 AND occurred_at < $2`, from, to); n != 0 {
		t.Fatalf("após detach, a tabela viva não deveria ter eventos da janela, veio %d", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM `+partition); n != 3 {
		t.Fatalf("a partição destacada deveria preservar os 3 eventos, veio %d", n)
	}
	// O arquivamento foi auditado (admin.mutation) na cadeia da org.
	if countAction(t, pool, org, "admin.mutation") < 1 {
		t.Fatalf("arquivamento deveria ter sido auditado")
	}

	// Restaura e confirma a cadeia íntegra (eventos de volta + evento de arquivo).
	if err := archiver.Restore(ctx, partition, from, to); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	verifier := NewAuditVerifier(pool, signer)
	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if !rep.OK {
		t.Fatalf("após restauração a cadeia deveria verificar: %+v", rep)
	}
}
