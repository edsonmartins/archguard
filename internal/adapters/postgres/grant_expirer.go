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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// GrantExpirer materializes the expiry of privileged grants whose window has
// passed and cascades the revocation to their derived sessions (pacote 004
// T-012). It runs per tenant. Note that authority is ALREADY denied at decision
// time for a past-window grant (domain.PrivilegedGrant.Authorizes); this job only
// materializes the state and revokes the derived sessions — it is not the thing
// standing between an expired grant and access.
type GrantExpirer struct {
	repo  *TenantRepository
	audit AuditEmitter
	now   func() time.Time
}

// NewGrantExpirer builds the expirer over the tenant repository and audit
// emitter. clock supplies the current instant.
func NewGrantExpirer(repo *TenantRepository, audit AuditEmitter, clock func() time.Time) *GrantExpirer {
	return &GrantExpirer{repo: repo, audit: audit, now: clock}
}

// ExpireDue expires every active grant of the tenant whose window has passed, in
// ONE transaction: each grant is moved to expired, its derived sessions are
// cascade-revoked, and an expiry event is audited — atomically, so a grant is
// never left expired without its sessions revoked (or vice versa). It returns how
// many grants were expired. The audit uses the SYSTEM principal set on the
// context by the caller (the scheduled job), so emitAudit does not fail closed on
// a missing principal.
func (e *GrantExpirer) ExpireDue(ctx context.Context) (int, error) {
	now := e.now()
	expired := 0
	err := e.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		grantStore := NewPrivilegedGrantStore(ttx)
		sessionStore := NewTenantSessionStore(ttx)

		due, err := grantStore.ListActiveExpired(ctx, now)
		if err != nil {
			return err
		}
		for i := range due {
			g := due[i]
			if err := g.Expire(now); err != nil {
				return fmt.Errorf("postgres: expiração da concessão %s: %w", g.ID, err)
			}
			if err := grantStore.SaveDecision(ctx, g); err != nil {
				return err
			}
			// Cascade: revoke the sessions derived from this grant.
			if _, err := sessionStore.RevokeByGrant(ctx, g.ID); err != nil {
				return err
			}
			if err := emitAudit(ctx, ttx.Tx(), e.audit, g.OrganizationID,
				domain.ActionPrivilegedGrantExpire,
				domain.AuditTarget{Type: "privileged_grant", ID: g.ID.String(), Label: "expiração de concessão"},
				"janela da concessão expirada"); err != nil {
				return err
			}
			expired++
		}
		return nil
	})
	return expired, err
}
