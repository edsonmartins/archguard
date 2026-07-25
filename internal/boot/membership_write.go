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

// membershipRevoker composes the L2 membership-revocation write: it establishes the
// acting principal (so the mutation is audited by a named actor, never anonymous —
// I-5.4), then delegates to postgres.MembershipRevoker, which in ONE tenant
// transaction revokes the membership (terminal, R4), ends the member's sessions in
// the tenant, and records a membership.revoke audit event. The AuditWriter is the
// emitter — this is the first write to the immutable trail from the console.
type membershipRevoker struct {
	pool  *pgxpool.Pool
	audit postgres.AuditEmitter
}

// newMembershipRevoker composes the revoker over the runtime pool and the audit
// writer (append-only, INV-2; nil clock defaults to time.Now).
func newMembershipRevoker(f *Factory) *membershipRevoker {
	return &membershipRevoker{pool: f.Pool(), audit: postgres.NewAuditWriter(f.Pool(), nil)}
}

// RevokeMembership revokes targetMembershipID in orgID on behalf of actor. It
// resolves the actor's opaque subject (the audit never carries the plaintext
// identity) and injects the principal before the audited transaction. A missing
// membership is surfaced distinctly (apihttp.ErrMembershipNotFound → 404).
func (r *membershipRevoker) RevokeMembership(ctx context.Context, actor apihttp.RevokeActor, orgID, targetMembershipID uuid.UUID) (int, error) {
	idn, err := postgres.NewIdentityStore(r.pool).Get(ctx, actor.IdentityID)
	if err != nil {
		return 0, err
	}
	principal := domain.AuditActor{
		IdentitySubject: idn.Subject,
		MembershipID:    actor.MembershipID,
		SessionID:       &actor.SessionID,
	}
	ctx = domain.WithPrincipal(ctx, principal)

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return 0, err
	}
	revoker := postgres.NewMembershipRevoker(postgres.NewTenantRepository(r.pool, scope), r.audit)
	_, sessions, err := revoker.RevokeMembership(ctx, targetMembershipID)
	if errors.Is(err, postgres.ErrMembershipNotFound) {
		return 0, apihttp.ErrMembershipNotFound
	}
	if err != nil {
		return 0, err
	}
	return sessions, nil
}
