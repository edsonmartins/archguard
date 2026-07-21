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

// Package globalaccess holds provisional implementations of the cross-tenant
// authorization and audit ports (domain.GlobalAuthorizer / domain.AccessAuditor).
//
// These are NOT the production controls: fine-grained authorization is OpenFGA
// (pacote 007) and the tamper-evident append-only audit trail is pacote 003.
// They exist so the GlobalRepository is usable and testable now, WITHOUT ever
// failing open (INV-6): the authorizer denies outside the dev profile, and the
// auditor is explicit about durability.
package globalaccess

import (
	"context"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
)

// ProfileAuthorizer permits cross-tenant access ONLY under the dev deployment
// profile, and denies everywhere else until the real PDP (OpenFGA, pacote 007) is
// wired. This is fail-closed (INV-6): pilot/production get no global access from
// this provisional authorizer — they must configure the real one.
type ProfileAuthorizer struct{}

// NewProfileAuthorizer builds the provisional profile-gated authorizer.
func NewProfileAuthorizer() *ProfileAuthorizer { return &ProfileAuthorizer{} }

// Authorize allows the access under the dev profile, otherwise denies.
func (a *ProfileAuthorizer) Authorize(_ context.Context, access domain.GlobalAccess) error {
	if err := access.Validate(); err != nil {
		return err
	}
	if deploy.Active() == deploy.Dev {
		return nil
	}
	return fmt.Errorf("%w: PDP de acesso cross-tenant não configurado no perfil %q (OpenFGA, pacote 007)",
		domain.ErrGlobalAccessDenied, deploy.Active())
}

var _ domain.GlobalAuthorizer = (*ProfileAuthorizer)(nil)

// MemoryAuditor is a PROVISIONAL, dev-only audit sink: it appends each access to
// an in-memory slice. It is NOT durable — records are lost on restart — and NOT
// tamper-evident, so it is not the production audit trail (pacote 003). It exists
// only so the GlobalRepository has a working AccessAuditor for dev and tests. In
// pilot/production the real append-only trail MUST be wired in its place.
type MemoryAuditor struct {
	mu      sync.Mutex
	records []domain.GlobalAccess
}

// NewMemoryAuditor builds an empty provisional in-memory auditor.
func NewMemoryAuditor() *MemoryAuditor { return &MemoryAuditor{} }

// Record appends the access. It validates first so an ill-formed access is not
// "recorded" as if it were a real event.
func (m *MemoryAuditor) Record(_ context.Context, access domain.GlobalAccess) error {
	if err := access.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, access)
	return nil
}

// Records returns a copy of what was recorded, for inspection in tests.
func (m *MemoryAuditor) Records() []domain.GlobalAccess {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.GlobalAccess, len(m.records))
	copy(out, m.records)
	return out
}

var _ domain.AccessAuditor = (*MemoryAuditor)(nil)
