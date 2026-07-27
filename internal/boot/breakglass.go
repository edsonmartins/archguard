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
func (b *breakglassRequester) RequestBreakglass(ctx context.Context, actor apihttp.RevokeActor, provenAAL domain.AAL, phishingResistant bool, orgID uuid.UUID, target domain.GrantTarget, justification, incidentRef string, notBefore, expiresAt time.Time) error {
	if actor.MembershipID == nil {
		return apihttp.ErrBreakglassInvalid
	}
	// Break-glass requires a phishing-resistant step-up (WebAuthn); TOTP does not qualify.
	// The L3 pipeline gate already enforced it — refuse EARLY here (before any grant is
	// created or any alert is emitted) if it was somehow bypassed. Defense-in-depth.
	if !phishingResistant || !provenAAL.AtLeast(domain.AAL2) {
		return apihttp.ErrBreakglassNeedsWebAuthn
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
	repo := postgres.NewTenantRepository(b.pool, scope)
	orch := postgres.NewBreakglassOrchestrator(repo, domain.NewBreakglassRequester(b.notifier), b.audit)
	grant, err := orch.Request(ctx, *actor.MembershipID, target, policy, justification, incidentRef, notBefore, expiresAt)
	switch {
	case err == nil:
		// created 'requested' + alerted + audited — advance past the step-up gate below.
	case errors.Is(err, domain.ErrNoNotificationChannel):
		return apihttp.ErrBreakglassChannelUnavailable
	case errors.Is(err, domain.ErrInvalidGrant):
		return apihttp.ErrBreakglassInvalid
	default:
		return err
	}
	// The pipeline already performed the WebAuthn step-up, so advance the fresh grant to
	// awaiting_approval (or active when zero approvals are required) using the session's
	// proven AAL. The pre-check above makes this fail only on a genuine inconsistency.
	switch err := postgres.NewPrivilegedAccessService(repo, b.audit).PassStepUp(ctx, grant.ID, provenAAL, phishingResistant); {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrStepUpNotPhishingResistant):
		return apihttp.ErrBreakglassNeedsWebAuthn
	default:
		return err
	}
}

// breakglassApprover records a peer approval on a break-glass grant (L3, T-008): it
// establishes the acting principal, then delegates to postgres.PrivilegedAccessService,
// which enforces the domain rules (distinct approver, NOT the requester, quorum → active)
// and records the approval audit ATOMICALLY with the state change (I-5.4). The approver is
// the caller's own membership (INV-1), never taken from the request.
type breakglassApprover struct {
	pool  *pgxpool.Pool
	audit postgres.AuditEmitter
}

// newBreakglassApprover composes the approver over the runtime pool and the audit writer.
func newBreakglassApprover(f *Factory) *breakglassApprover {
	return &breakglassApprover{pool: f.Pool(), audit: postgres.NewAuditWriter(f.Pool(), nil)}
}

// ApproveBreakglass records actor's approval on grantID in orgID. Separation of duties and
// the quorum are enforced by the domain: the requester approving is apihttp.ErrSelfApproval
// (403), a repeat approver apihttp.ErrDuplicateApproval (409), a grant not awaiting approval
// apihttp.ErrGrantNotActive (409), an absent grant apihttp.ErrGrantNotFound (404).
func (a *breakglassApprover) ApproveBreakglass(ctx context.Context, actor apihttp.RevokeActor, orgID, grantID uuid.UUID) error {
	if actor.MembershipID == nil {
		return apihttp.ErrGrantNotActive
	}
	idn, err := postgres.NewIdentityStore(a.pool).Get(ctx, actor.IdentityID)
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
	svc := postgres.NewPrivilegedAccessService(postgres.NewTenantRepository(a.pool, scope), a.audit)
	switch _, err := svc.Approve(ctx, grantID, *actor.MembershipID); {
	case err == nil:
		return nil
	case errors.Is(err, postgres.ErrGrantNotFound):
		return apihttp.ErrGrantNotFound
	case errors.Is(err, domain.ErrSelfApproval):
		return apihttp.ErrSelfApproval
	case errors.Is(err, domain.ErrGrantDuplicateApproval):
		return apihttp.ErrDuplicateApproval
	case errors.Is(err, domain.ErrGrantTransition):
		return apihttp.ErrGrantNotActive
	default:
		return err
	}
}
