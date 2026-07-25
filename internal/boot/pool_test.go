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

package boot

import (
	"context"
	"os"
	"testing"
)

// resetPool restores the package singleton between tests so ordering does not
// leak state. Tests here run in one process and share the global pool.
func resetPool() {
	poolMu.Lock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
	poolMu.Unlock()
}

func TestPoolNilBeforeInit(t *testing.T) {
	resetPool()
	if Pool() != nil {
		t.Fatalf("Pool() should be nil before InitPool")
	}
}

func TestClosePoolSafeWhenUnset(t *testing.T) {
	resetPool()
	// Must not panic when there is no pool, so it can be deferred blindly.
	ClosePool()
	if Pool() != nil {
		t.Fatalf("Pool() should remain nil after ClosePool with no pool")
	}
}

func TestInitPoolEmptyDSNFailsClosed(t *testing.T) {
	resetPool()
	err := InitPool(context.Background(), "")
	if err == nil {
		t.Fatalf("InitPool with empty DSN should return an error (fail-closed)")
	}
	if Pool() != nil {
		t.Fatalf("Pool() should stay nil when InitPool fails")
	}
}

// TestInitPoolLifecycle exercises the real open/close cycle against Postgres. It
// requires ARCHGUARD_TEST_DSN, matching the integration convention in the repo;
// it is skipped (not faked) when no database is configured.
func TestInitPoolLifecycle(t *testing.T) {
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN not set; skipping runtime pool integration test")
	}
	resetPool()
	t.Cleanup(resetPool)

	ctx := context.Background()
	if err := InitPool(ctx, dsn); err != nil {
		t.Fatalf("InitPool: %v", err)
	}
	p := Pool()
	if p == nil {
		t.Fatalf("Pool() should return the open pool after InitPool")
	}
	if err := p.Ping(ctx); err != nil {
		t.Fatalf("pool Ping after InitPool: %v", err)
	}

	// Idempotent: a second InitPool keeps the same pool.
	if err := InitPool(ctx, dsn); err != nil {
		t.Fatalf("second InitPool should be a no-op: %v", err)
	}
	if Pool() != p {
		t.Fatalf("second InitPool must not replace the live pool")
	}

	ClosePool()
	if Pool() != nil {
		t.Fatalf("Pool() should be nil after ClosePool")
	}
}
