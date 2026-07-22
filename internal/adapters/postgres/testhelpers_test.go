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

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedLegacyTables creates the minimal legacy tables (organization, role) that
// the XORM Sync2 would create before migrations run at boot. Migrations
// 0003/0008 extend them with stable UUID ids, which the membership and
// role_assignment FKs target — so the store integration tests must stand them up
// first, mirroring the real boot order (RUNBOOK: migrations run after Sync2).
func seedLegacyTables(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS organization (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
		`CREATE TABLE IF NOT EXISTS role (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("seed legacy: %v", err)
		}
	}
}
