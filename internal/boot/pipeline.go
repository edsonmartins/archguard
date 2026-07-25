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
	"net/http"
	"sync"

	"github.com/casdoor/casdoor/internal/adapters/globalaccess"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pipeline holds the assurance-enforcement pieces every mounted domain handler is
// wrapped with: the operation catalog (INV-8: each operation classified L1/L2/L3)
// and the middleware that resolves the session and enforces the classification,
// emitting a step-up challenge on insufficient assurance (ADR-0010).
//
// The session RESOLVER is the real BridgingResolver over the postgres SessionBridge;
// the tenant floor is the real OrgPolicyAuthority (org_mfa_policy). The identity+
// session binding is read from the request context (contextBinding); until the
// login bridge (T-004b) populates it, every domain operation resolves "not logged
// in" and is denied — the pipeline is fail-closed by construction.
type Pipeline struct {
	catalog *domain.OperationCatalog
	mw      *apihttp.AssuranceMiddleware
}

// NewPipeline assembles the assurance pipeline over the runtime pool.
//
// The cross-tenant global-access chain used by SessionBridge (GlobalAuthorizer /
// AccessAuditor) is wired to the PROVISIONAL dev adapters here: ProfileAuthorizer
// denies cross-tenant access outside the dev profile (fail-closed), and MemoryAuditor
// is a non-durable dev sink. That chain is only exercised once a session resolves,
// which requires the login bridge (T-004b); wiring the durable authorizer (OpenFGA,
// pacote 007) and auditor (trilha, pacote 003) for conformant profiles is part of
// activating sessions (T-004b) — not reached at runtime until then.
func NewPipeline(pool *pgxpool.Pool) *Pipeline {
	global := postgres.NewGlobalRepository(pool, globalaccess.NewProfileAuthorizer(), globalaccess.NewMemoryAuditor())
	loader := postgres.NewSessionBridge(pool, global)
	resolver := apihttp.NewBridgingResolver(contextBinding{}, loader)
	floor := postgres.NewOrgPolicyAuthority(pool)

	catalog := domain.NewOperationCatalog()
	guard := domain.NewAssuranceGuard(catalog)
	mw := apihttp.NewAssuranceMiddleware(guard, resolver, floor)

	return &Pipeline{catalog: catalog, mw: mw}
}

// RegisterOperation classifies an operation (INV-8). The mount tasks (T-005+) call
// this at boot for each operation they expose, before the server serves. The guard
// reads the catalog live, so registrations made before serving are all visible.
func (p *Pipeline) RegisterOperation(op domain.Operation) error {
	return p.catalog.Register(op)
}

// Require wraps a domain handler so it runs only when the request's session
// satisfies the operation's classified level. An unregistered operation fails
// closed (the middleware denies), so a handler wrapped with an unclassified id is
// never served — the contract T-016's build gate enforces.
func (p *Pipeline) Require(operationID string, next http.Handler) http.Handler {
	return p.mw.Require(operationID, next)
}

// The active pipeline is a boot singleton, built once (InitPipeline) and consulted
// by the mount tasks to register operations and wrap handlers.
var (
	pipelineMu     sync.Mutex
	activePipeline *Pipeline
)

// InitPipeline builds the assurance pipeline over the runtime pool and stores it.
// Must run at boot after InitPool and before the mount tasks register handlers.
func InitPipeline(pool *pgxpool.Pool) {
	pipelineMu.Lock()
	defer pipelineMu.Unlock()
	activePipeline = NewPipeline(pool)
}

// ActivePipeline returns the boot pipeline, or nil if InitPipeline has not run.
func ActivePipeline() *Pipeline {
	pipelineMu.Lock()
	defer pipelineMu.Unlock()
	return activePipeline
}
