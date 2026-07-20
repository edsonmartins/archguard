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
	"strings"
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

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("conexão: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

// seedLegacyOrganization creates the minimal `organization` table the way the
// XORM Sync2 would before migrations run at boot (composite string PK). Migration
// 0003 then extends it with the stable UUID id, and 0004's membership FK targets
// that id — so the migrator's own tests must stand this up first, exactly as the
// real boot order guarantees (RUNBOOK: migrations run after Sync2).
func seedLegacyOrganization(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	_, err := conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS organization (
			owner text NOT NULL,
			name  text NOT NULL,
			PRIMARY KEY (owner, name)
		)`)
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
}

// TestRunAppliesMigrationsIdempotently proves Run applies the embedded
// migrations against a real database, records them in schema_migrations, and is
// a no-op on a second call (advisory-lock-serialized, versioned).
func TestRunAppliesMigrationsIdempotently(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()
	seedLegacyOrganization(t, connect(t, dsn))

	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run (1ª vez): %v", err)
	}
	// Second call must be a clean no-op: nothing pending.
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run (2ª vez, idempotente): %v", err)
	}

	conn := connect(t, dsn)

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

	// The new schema must exist: identity + membership tables and the stable
	// organization.id column (migrations 0002/0003/0004).
	for _, obj := range []struct {
		q, what string
	}{
		{"SELECT to_regclass('public.identity') IS NOT NULL", "tabela identity"},
		{"SELECT to_regclass('public.membership') IS NOT NULL", "tabela membership"},
		{`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name='organization' AND column_name='id')`, "coluna organization.id"},
	} {
		var ok bool
		if err := conn.QueryRow(ctx, obj.q).Scan(&ok); err != nil {
			t.Fatalf("checagem de %s: %v", obj.what, err)
		}
		if !ok {
			t.Errorf("%s não foi criada pelas migrations", obj.what)
		}
	}
}

// TestRunCreatesIdentityConstraints proves the identity table rejects the values
// the domain forbids: an out-of-set type/status and a duplicate subject. The
// database is the second barrier — it must not trust the application.
func TestRunCreatesIdentityConstraints(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()
	seedLegacyOrganization(t, connect(t, dsn))
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run: %v", err)
	}
	conn := connect(t, dsn)

	subj := "it-subject-" + t.Name()
	if _, err := conn.Exec(ctx, "DELETE FROM identity WHERE subject = $1", subj); err != nil {
		t.Fatalf("limpeza prévia: %v", err)
	}
	if _, err := conn.Exec(ctx,
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'human')", subj); err != nil {
		t.Fatalf("insert válido falhou: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DELETE FROM identity WHERE subject LIKE $1", subj+"%")
	})

	mustReject(t, conn, "type inválido",
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'robot')", subj+"-bad")
	mustReject(t, conn, "status inválido",
		"INSERT INTO identity (id, subject, type, status) VALUES (gen_random_uuid(), $1, 'human', 'deleted')", subj+"-bad2")
	mustReject(t, conn, "subject duplicado",
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'service')", subj)
}

// TestRunCreatesMembershipConstraints proves the membership table enforces the
// FKs to identity/organization, the status CHECK, and the (identity_id,
// organization_id) uniqueness (RFC-0002 R3) — the schema-level guarantees behind
// the domain state machine.
func TestRunCreatesMembershipConstraints(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()
	seedLegacyOrganization(t, connect(t, dsn))
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run: %v", err)
	}
	conn := connect(t, dsn)

	// An organization row auto-gets a stable id via the column default (0003).
	orgName := "it-org-" + t.Name()
	var orgID string
	if err := conn.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", orgName).Scan(&orgID); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	subj := "it-msubject-" + t.Name()
	var idnID string
	if err := conn.QueryRow(ctx,
		"INSERT INTO identity (id, subject, type) VALUES (gen_random_uuid(), $1, 'human') RETURNING id", subj).Scan(&idnID); err != nil {
		t.Fatalf("insert identity: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = conn.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
		_, _ = conn.Exec(bg, "DELETE FROM identity WHERE subject = $1", subj)
		_, _ = conn.Exec(bg, "DELETE FROM organization WHERE name = $1", orgName)
	})

	// Valid membership inserts.
	if _, err := conn.Exec(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'active')",
		idnID, orgID); err != nil {
		t.Fatalf("insert membership válido: %v", err)
	}

	mustReject(t, conn, "status fora do CHECK",
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'pending')",
		idnID, orgID)
	mustReject(t, conn, "status NULL (sem default)",
		"INSERT INTO membership (id, identity_id, organization_id) VALUES (gen_random_uuid(), $1, $2)",
		idnID, orgID)
	mustReject(t, conn, "identity_id inexistente (FK)",
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), gen_random_uuid(), $1, 'active')",
		orgID)
	mustReject(t, conn, "organization_id inexistente (FK)",
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, gen_random_uuid(), 'active')",
		idnID)
	mustReject(t, conn, "par (identity_id, organization_id) duplicado (R3)",
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'invited')",
		idnID, orgID)
}

// TestPersonalColumnsAreLGPDClassified proves the personal-data columns carry a
// LGPD classification in the catalog (migration 0006), the I-3.3 requirement:
// categoria, finalidade, base legal e retenção declaradas no modelo de dados.
func TestPersonalColumnsAreLGPDClassified(t *testing.T) {
	dsn := dsnFromEnv(t)
	ctx := context.Background()
	seedLegacyOrganization(t, connect(t, dsn))
	if err := Run(ctx, dsn); err != nil {
		t.Fatalf("Run: %v", err)
	}
	conn := connect(t, dsn)

	for _, col := range []struct{ table, column string }{
		{"identity", "primary_email_enc"},
		{"identity", "email_hash"},
		{"identity", "display_name_enc"},
		{"membership", "attributes_enc"},
	} {
		var comment *string
		err := conn.QueryRow(ctx, `
			SELECT col_description(($1||'.'||$2)::regclass, a.attnum)
			FROM pg_attribute a
			WHERE a.attrelid = ($1||'.'||$2)::regclass AND a.attname = $3`,
			"public", col.table, col.column).Scan(&comment)
		if err != nil {
			t.Fatalf("%s.%s: leitura do comentário: %v", col.table, col.column, err)
		}
		if comment == nil || !strings.HasPrefix(*comment, "LGPD |") {
			t.Errorf("%s.%s sem classificação LGPD (I-3.3): %v", col.table, col.column, comment)
		}
		for _, needle := range []string{"categoria=", "finalidade=", "base_legal=", "retencao="} {
			if comment == nil || !strings.Contains(*comment, needle) {
				t.Errorf("%s.%s: classificação LGPD sem %q", col.table, col.column, needle)
			}
		}
	}
}

// mustReject asserts that an INSERT/UPDATE is rejected by the database. It runs
// the statement in its own subtransaction so the aborted statement does not
// poison the shared connection for later assertions.
func mustReject(t *testing.T, conn *pgx.Conn, what, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("%s: begin: %v", what, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, sql, args...); err == nil {
		t.Errorf("%s: deveria ter sido rejeitado pelo banco, foi aceito", what)
	}
}
