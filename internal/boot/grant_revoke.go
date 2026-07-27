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
	"errors"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// grantRevoker composes the L3 privileged-grant revocation write: it establishes the
// acting principal (so the mutation is audited by a named actor, never anonymous —
// I-5.4), then delegates to postgres.PrivilegedAccessService, which in ONE tenant
// transaction revokes the grant, cascade-revokes its derived sessions and records a
// privileged.grant.revoke audit event — atomically (fail-closed, I-5.4). The grant is
// fetched under the tenant's RLS, so a grant of another org is simply not found (INV-5).
// The AuditWriter is the append-only emitter (INV-2).
type grantRevoker struct {
	pool  *pgxpool.Pool
	audit postgres.AuditEmitter
}

// newGrantRevoker composes the revoker over the runtime pool and the audit writer
// (append-only, INV-2; nil clock defaults to time.Now).
func newGrantRevoker(f *Factory) *grantRevoker {
	return &grantRevoker{pool: f.Pool(), audit: postgres.NewAuditWriter(f.Pool(), nil)}
}

// RevokeGrant revokes grantID in orgID on behalf of actor. It resolves the actor's
// opaque subject (the audit never carries the plaintext identity) and injects the
// principal before the audited transaction. An absent grant (including one from another
// org, filtered by the tenant RLS) is surfaced as apihttp.ErrGrantNotFound (404); a
// non-active grant (already revoked/expired) as apihttp.ErrGrantNotActive (409).
func (r *grantRevoker) RevokeGrant(ctx context.Context, actor apihttp.RevokeActor, orgID, grantID uuid.UUID) error {
	idn, err := postgres.NewIdentityStore(r.pool).Get(ctx, actor.IdentityID)
	if err != nil {
		return err
	}
	principal := domain.AuditActor{
		IdentitySubject: idn.Subject,
		MembershipID:    actor.MembershipID,
		SessionID:       &actor.SessionID,
	}
	ctx = domain.WithPrincipal(ctx, principal)

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	svc := postgres.NewPrivilegedAccessService(postgres.NewTenantRepository(r.pool, scope), r.audit)
	switch err := svc.Revoke(ctx, grantID); {
	case err == nil:
		return nil
	case errors.Is(err, postgres.ErrGrantNotFound):
		return apihttp.ErrGrantNotFound
	case errors.Is(err, domain.ErrGrantTransition):
		return apihttp.ErrGrantNotActive
	default:
		return err
	}
}
