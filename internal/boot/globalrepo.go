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
	"github.com/casdoor/casdoor/internal/adapters/globalaccess"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newGlobalRepository builds the cross-tenant repository with the REAL, durable
// controls (ADR-0022) — the single place the boot wires them, replacing the
// per-call-site provisional pair. The authorizer is the Go ScopedAuthorizer (no
// external dependency, so the login path honors I-1.3): it permits self-confined
// reads in any profile and fails closed on broad cross-tenant reads in conformant
// profiles. The auditor is the durable AccessAuditor (append-only global_access_audit,
// migration 0035), so every cross-tenant access is recorded before it runs (I-5.4).
//
// The in-memory provisional pair (ProfileAuthorizer/MemoryAuditor) is now only for
// unit tests that run without a pool — never the boot path.
func newGlobalRepository(pool *pgxpool.Pool) *postgres.GlobalRepository {
	return postgres.NewGlobalRepository(pool, globalaccess.NewScopedAuthorizer(), postgres.NewAccessAuditor(pool))
}
