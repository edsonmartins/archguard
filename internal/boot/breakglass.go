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
	"time"

	"github.com/casdoor/casdoor/internal/adapters/notification"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// breakglassRequester composes the L3 break-glass REQUEST write: it establishes the acting
// principal (so the mutation is audited by a named actor — I-5.4), applies the break-glass
// policy by deployment profile, and delegates to postgres.BreakglassOrchestrator, which is
// fail-closed on BOTH the notification channel (alert at request time, before any grant)
// and the audit (atomic with the grant Create). The subject is the requesting operator's
// own membership (INV-1), never taken from the request. The AuditWriter is the append-only
// emitter (INV-2).
type breakglassRequester struct {
	pool              *pgxpool.Pool
	audit             postgres.AuditEmitter
	notifier          domain.Notifier
	requiredApprovals int
	production        bool
}

// newBreakglassRequester composes the requester over the runtime pool, the provider-backed
// notifier (fail-closed channel), the audit writer, and the break-glass policy DERIVED from
// the deployment profile: production requires the default quorum of distinct approvers
// (DefaultBreakglassApprovals); the dev profile relaxes to a single approver (and denies L3
// outright at the pipeline anyway).
func newBreakglassRequester(f *Factory) *breakglassRequester {
	approvals := domain.DefaultBreakglassApprovals
	if f.Profile().IsDev() {
		approvals = 1
	}
	return &breakglassRequester{
		pool:              f.Pool(),
		audit:             postgres.NewAuditWriter(f.Pool(), nil),
		notifier:          notification.NewProviderNotifier(),
		requiredApprovals: approvals,
		production:        f.Profile() == deploy.Production,
	}
}

// RequestBreakglass opens a break-glass grant for the actor over target in orgID. It
// resolves the actor's opaque subject (the audit never carries the plaintext identity),
// injects the principal, and requests the grant with the subject being the actor's own
// membership (INV-1). A missing notification channel is surfaced as
// apihttp.ErrBreakglassChannelUnavailable (503); a domain validation failure as
// apihttp.ErrBreakglassInvalid (422).
func (b *breakglassRequester) RequestBreakglass(ctx context.Context, actor apihttp.RevokeActor, orgID uuid.UUID, target domain.GrantTarget, justification, incidentRef string, notBefore, expiresAt time.Time) error {
	if actor.MembershipID == nil {
		return apihttp.ErrBreakglassInvalid
	}
	idn, err := postgres.NewIdentityStore(b.pool).Get(ctx, actor.IdentityID)
	if err != nil {
		return err
	}
	principal := domain.AuditActor{
		IdentitySubject: idn.Subject,
		MembershipID:    actor.MembershipID,
		SessionID:       &actor.SessionID,
	}
	ctx = domain.WithPrincipal(ctx, principal)

	policy, err := domain.NewBreakglassPolicy(b.requiredApprovals, b.production)
	if err != nil {
		return err
	}
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	orch := postgres.NewBreakglassOrchestrator(
		postgres.NewTenantRepository(b.pool, scope),
		domain.NewBreakglassRequester(b.notifier),
		b.audit,
	)
	switch _, err := orch.Request(ctx, *actor.MembershipID, target, policy, justification, incidentRef, notBefore, expiresAt); {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNoNotificationChannel):
		return apihttp.ErrBreakglassChannelUnavailable
	case errors.Is(err, domain.ErrInvalidGrant):
		return apihttp.ErrBreakglassInvalid
	default:
		return err
	}
}
