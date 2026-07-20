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

package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// dsnFromEnv returns the DSN for the live-database integration tests, or skips
// the test when it is unset — so a machine without PostgreSQL (typical CI on the
// tools/unit path) runs the rest of the suite unaffected. Point it at a THROWAWAY
// database: these tests apply real migrations.
func dsnFromEnv(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — pulando teste de integração de migrations")
	}
	return dsn
}

// TestRunAppliesMigrationsIdempotently proves Run applies the embedded
// migrations against a real database, records them in schema_migrations, and is
// a no-op on a second call (advisory-lock-serialized, versioned).
func TestRunAppliesMigrationsIdempotently(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()

	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run (1ª vez): %v", err)
	}
	// Second call must be a clean no-op: nothing pending.
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run (2ª vez, idempotente): %v", err)
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("conexão: %v", err)
	}
	defer conn.Close(ctx)

	// Every embedded migration must be recorded exactly once.
	want, err := parseMigrations(migrationsFS)
	if err != nil {
		t.Fatalf("parseMigrations: %v", err)
	}
	var applied int
	if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatalf("contagem de schema_migrations: %v", err)
	}
	if applied != len(want) {
		t.Fatalf("schema_migrations tem %d entradas, quer %d", applied, len(want))
	}

	// Migration 0002 must have created `identity` with its CHECK/UNIQUE shape.
	var exists bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'identity'
		)`).Scan(&exists); err != nil {
		t.Fatalf("checagem de tabela identity: %v", err)
	}
	if !exists {
		t.Fatal("tabela identity não foi criada pela migration 0002")
	}
}

// TestRunCreatesIdentityConstraints proves the identity table rejects the values
// the domain forbids: an out-of-set type/status and a duplicate subject. The
// database is the second barrier — it must not trust the application.
func TestRunCreatesIdentityConstraints(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run: %v", err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("conexão: %v", err)
	}
	defer conn.Close(ctx)

	// Isolate this test's rows so a re-run against the same DB is clean.
	subj := "it-subject-" + t.Name()
	if _, err := conn.Exec(ctx, "DELETE FROM identity WHERE subject = $1", subj); err != nil {
		t.Fatalf("limpeza prévia: %v", err)
	}

	// A valid row inserts.
	if _, err := conn.Exec(ctx,
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'human')", subj); err != nil {
		t.Fatalf("insert válido falhou: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DELETE FROM identity WHERE subject = $1", subj)
	})

	// Invalid type must violate the CHECK.
	if _, err := conn.Exec(ctx,
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'robot')", subj+"-bad"); err == nil {
		t.Error("type inválido 'robot' deveria violar o CHECK")
		_, _ = conn.Exec(ctx, "DELETE FROM identity WHERE subject = $1", subj+"-bad")
	}
	// Invalid status must violate the CHECK.
	if _, err := conn.Exec(ctx,
		"INSERT INTO identity (id, subject, type, status) VALUES (gen_random_uuid(), $1, 'human', 'deleted')", subj+"-bad2"); err == nil {
		t.Error("status inválido 'deleted' deveria violar o CHECK")
		_, _ = conn.Exec(ctx, "DELETE FROM identity WHERE subject = $1", subj+"-bad2")
	}
	// Duplicate subject must violate the UNIQUE constraint.
	if _, err := conn.Exec(ctx,
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'service')", subj); err == nil {
		t.Error("subject duplicado deveria violar a UNIQUE constraint")
	}
}
