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

// Package boot is the ArchGuard composition root: the infrastructure layer that
// owns the runtime pgx pool, selects adapters by deployment profile and mounts
// the internal/http handlers onto the server (pacote 011). It is allowed to
// import the framework and the driver; internal/domain must not (INV-3).
package boot

import (
	"context"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The runtime pool is a process-lifetime singleton owned by the composition
// root. It is guarded so InitPool/Pool/ClosePool are safe from the boot goroutine
// and any request handler that reads it.
var (
	poolMu sync.RWMutex
	pool   *pgxpool.Pool
)

// InitPool opens the single runtime pgx pool from the given data source name and
// stores it for the composition root. The DSN is passed in (not read from the
// framework config) so the boot layer stays decoupled and testable, mirroring
// internal/migrate. It must run AFTER migrations — the schema has to exist — and
// the caller must invoke ClosePool on shutdown.
//
// Idempotent: a second call while a pool is already open is a no-op. An empty DSN
// or a pool that cannot be reached is a fatal boot error (fail-closed, INV-6):
// the composition root cannot serve without its database.
func InitPool(ctx context.Context, dsn string) error {
	poolMu.Lock()
	defer poolMu.Unlock()
	if pool != nil {
		return nil
	}
	if dsn == "" {
		return fmt.Errorf("boot: empty data source name; cannot open the runtime pool")
	}
	p, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("boot: open runtime pool: %w", err)
	}
	pool = p
	return nil
}

// Pool returns the runtime pool, or nil if InitPool has not run. The composition
// root passes it to the postgres stores and repositories.
func Pool() *pgxpool.Pool {
	poolMu.RLock()
	defer poolMu.RUnlock()
	return pool
}

// ClosePool closes the runtime pool if one is open and clears it. Safe to call
// when no pool exists, so it can be deferred unconditionally at shutdown.
func ClosePool() {
	poolMu.Lock()
	defer poolMu.Unlock()
	if pool == nil {
		return
	}
	pool.Close()
	pool = nil
}
