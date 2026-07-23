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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// BreakglassOrchestrator composes the break-glass REQUEST flow end to end,
// fail-closed on BOTH preconditions (ADR-0008 / T-013): a request with no
// notification channel is denied (via the domain requester) and a request whose
// audit cannot be durably recorded is rolled back (via the atomic audit). It is
// the seam where the domain rules (notify-available, alert-at-request, build the
// grant) meet persistence and the tamper-evident trail.
type BreakglassOrchestrator struct {
	repo      *TenantRepository
	requester *domain.BreakglassRequester
	audit     AuditEmitter
}

// NewBreakglassOrchestrator builds the orchestrator over the tenant repository,
// the domain requester (which holds the notifier) and the audit emitter.
func NewBreakglassOrchestrator(repo *TenantRepository, requester *domain.BreakglassRequester, audit AuditEmitter) *BreakglassOrchestrator {
	return &BreakglassOrchestrator{repo: repo, requester: requester, audit: audit}
}

// Request opens a break-glass grant. Order matters for fail-closed:
//
//  1. the domain requester checks the notification channel is available and emits
//     the real-time alert (outside any DB transaction — a remote call must not run
//     inside a tx, RFC-0004 §4); a missing channel or an undeliverable alert deny
//     here, before any grant is persisted;
//  2. the grant and its request audit event are then written in ONE transaction —
//     if the audit cannot be recorded (no principal, audit unavailable) the whole
//     transaction rolls back, so a break-glass whose request is not auditable
//     never persists (I-5.4).
//
// A spurious alert (sent in step 1, then the tx rolls back) is the safe failure
// direction: an unalerted or unaudited emergency access is not.
func (o *BreakglassOrchestrator) Request(ctx context.Context, subjectMembershipID uuid.UUID, target domain.GrantTarget, policy domain.BreakglassPolicy, justification, incidentRef string, notBefore, expiresAt time.Time) (domain.PrivilegedGrant, error) {
	org := o.repo.Scope().OrganizationID()
	g, err := o.requester.Request(ctx, org, subjectMembershipID, target, policy, justification, incidentRef, notBefore, expiresAt)
	if err != nil {
		return domain.PrivilegedGrant{}, err
	}
	err = o.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		if err := NewPrivilegedGrantStore(ttx).Create(ctx, g); err != nil {
			return err
		}
		return emitAudit(ctx, ttx.Tx(), o.audit, org, domain.ActionBreakglassRequest,
			domain.AuditTarget{Type: "privileged_grant", ID: g.ID.String(), Label: "solicitação de break-glass"},
			"solicitação de break-glass, incidente "+incidentRef)
	})
	if err != nil {
		return domain.PrivilegedGrant{}, err
	}
	return g, nil
}
