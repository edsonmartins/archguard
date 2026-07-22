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

package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// O CLI sai 0 numa trilha íntegra e 1 quando há adulteração — o sinal para
// cron/CI alertar pelo código de saída.
func TestAuditVerifyCLIExitCodes(t *testing.T) {
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — CLI exige PostgreSQL real")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// Sobe o esquema (Sync2 legado + migrations), como o boot real.
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS organization (owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
		`CREATE TABLE IF NOT EXISTS role (owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("seed legado: %v", err)
		}
	}
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	org := uuid.New()
	w := postgres.NewAuditWriter(pool, nil)
	for i := 0; i < 3; i++ {
		if _, err := w.Append(ctx, domain.AuditEventInput{
			OrganizationID: org,
			Action:         domain.ActionAuthLogin,
			Actor:          domain.AuditActor{IdentitySubject: "sub-cli"},
			Outcome:        domain.Allowed,
		}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		conn, err := pool.Acquire(bg)
		if err == nil {
			defer conn.Release()
			_, _ = conn.Exec(bg, "SET session_replication_role = replica")
			_, _ = conn.Exec(bg, "DELETE FROM audit_event WHERE organization_id = $1", org.String())
			_, _ = conn.Exec(bg, "DELETE FROM audit_chain_head WHERE organization_id = $1", org.String())
			_, _ = conn.Exec(bg, "SET session_replication_role = origin")
		}
	})

	// Íntegra ⇒ exit 0.
	if code := run([]string{"-dsn", dsn, "-org", org.String()}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("trilha íntegra: exit = %d, quero 0", code)
	}

	// Adultera um evento (bypass superusuário) ⇒ exit 1.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	_, _ = conn.Exec(ctx, "SET session_replication_role = replica")
	if _, err := conn.Exec(ctx,
		"UPDATE audit_event SET reason = 'adulterado' WHERE organization_id = $1 AND seq = 2", org.String()); err != nil {
		conn.Release()
		t.Fatalf("adultera: %v", err)
	}
	_, _ = conn.Exec(ctx, "SET session_replication_role = origin")
	conn.Release()

	if code := run([]string{"-dsn", dsn, "-org", org.String()}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("trilha adulterada: exit = %d, quero 1", code)
	}
}
