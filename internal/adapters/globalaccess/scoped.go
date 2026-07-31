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

package globalaccess

import (
	"context"
	"fmt"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
)

// ScopedAuthorizer is the REAL cross-tenant authorizer (ADR-0022) — a Go evaluator,
// with NO external dependency, so the login path never depends on a remote service
// (I-1.3). It decides by the access SCOPE:
//
//   - ScopeSelf — a read confined to the principal's own identity (login/console
//     resolving one's own memberships). Permitted in ANY profile: it is intrinsic
//     to authentication, and the INV-1 guarantee already lives in the call-site,
//     which reads only the authenticated identity.
//   - ScopeCrossTenant — a broad read across tenants. FAIL-CLOSED in a conformant
//     profile (INV-6) until a real cross-tenant policy (operator role) is wired;
//     permitted only under the dev profile.
//
// Fine-grained resource authorization remains OpenFGA's job (the PolicyDecisionPoint
// port, pacote 007) — a DISTINCT concern from this cross-tenant read gate.
type ScopedAuthorizer struct{}

// NewScopedAuthorizer builds the real scope-based authorizer.
func NewScopedAuthorizer() *ScopedAuthorizer { return &ScopedAuthorizer{} }

// Authorize permits self-confined access always, and broad cross-tenant access only
// under the dev profile (fail-closed elsewhere).
func (a *ScopedAuthorizer) Authorize(_ context.Context, access domain.GlobalAccess) error {
	if err := access.Validate(); err != nil {
		return err
	}
	if access.Scope == domain.ScopeSelf {
		return nil
	}
	// Broad cross-tenant read: no real policy yet outside dev — deny (never open).
	if deploy.Active() == deploy.Dev {
		return nil
	}
	return fmt.Errorf("%w: leitura cross-tenant ampla sem política no perfil %q (ADR-0022)",
		domain.ErrGlobalAccessDenied, deploy.Active())
}

var _ domain.GlobalAuthorizer = (*ScopedAuthorizer)(nil)
