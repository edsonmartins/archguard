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

// Package migrate applies versioned, ordered, idempotent SQL migrations to the
// PostgreSQL backend (ADR-0009). Concurrency across replicas is serialized by a
// PostgreSQL session advisory lock, so only one instance applies migrations at
// a time. New persistence uses pgx and explicit SQL (ADR-0016).
package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// advisoryLockKey is a fixed, arbitrary key that all ArchGuard instances use to
// serialize migration runs (pg_advisory_lock). It must never change.
const advisoryLockKey int64 = 728_411_003

// migration is one ordered, versioned unit of schema change.
type migration struct {
	version int
	name    string
	sql     string
}

var fileName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// parseMigrations reads and validates the embedded migration files, returning
// them ordered by version. It rejects malformed names and duplicate versions so
// the ordering can never be ambiguous.
func parseMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migrate: nome de migration inválido %q — use NNNN_nome.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migrate: versão %04d duplicada (%s e %s)", version, prev, e.Name())
		}
		seen[version] = e.Name()
		body, rerr := fs.ReadFile(fsys, "migrations/"+e.Name())
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, migration{version: version, name: m[2], sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// pending returns the migrations whose version is greater than the current max
// applied version.
func pending(all []migration, applied int) []migration {
	var out []migration
	for _, m := range all {
		if m.version > applied {
			out = append(out, m)
		}
	}
	return out
}

// Run applies all pending migrations against the PostgreSQL database at dsn.
// It is safe to call concurrently from multiple instances: the advisory lock
// ensures only one applies at a time and the rest observe the result.
func Run(ctx context.Context, dsn string) error {
	migrations, err := parseMigrations(migrationsFS)
	if err != nil {
		return err
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migrate: conexão falhou: %w", err)
	}
	defer conn.Close(ctx)

	// Serialize migration runs across replicas. The lock is held for the whole
	// session and released on Close.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("migrate: advisory lock falhou: %w", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockKey) }()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    integer     PRIMARY KEY,
			name       text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("migrate: criação de schema_migrations falhou: %w", err)
	}

	var applied int
	if err := conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&applied); err != nil {
		return fmt.Errorf("migrate: leitura de versão falhou: %w", err)
	}

	for _, m := range pending(migrations, applied) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin %04d falhou: %w", m.version, err)
		}
		if _, err := tx.Exec(ctx, m.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: aplicação de %04d_%s falhou: %w", m.version, m.name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.version, m.name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: registro de %04d falhou: %w", m.version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit de %04d falhou: %w", m.version, err)
		}
	}
	return nil
}

// dsnRedacted is a small helper for safe logging of a DSN (never log secrets).
func dsnRedacted(dsn string) string {
	parts := strings.Fields(dsn)
	for i, p := range parts {
		if strings.HasPrefix(p, "password=") {
			parts[i] = "password=***"
		}
	}
	return strings.Join(parts, " ")
}
