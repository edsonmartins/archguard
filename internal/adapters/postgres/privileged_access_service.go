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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// PrivilegedAccessService audits the remaining break-glass cycle events —
// approval, revocation and post-use review (T-017) — each ATOMICALLY with the
// state change (request and expiry are audited by their own flows, T-012/T-013).
// Every mutation shares the operation's transaction with its audit event, so an
// unaudited approval/revocation/review can never commit (I-5.4). Actor is the
// acting principal from the context.
type PrivilegedAccessService struct {
	repo  *TenantRepository
	audit AuditEmitter
}

// NewPrivilegedAccessService builds the service over the tenant repository and
// the audit emitter.
func NewPrivilegedAccessService(repo *TenantRepository, audit AuditEmitter) *PrivilegedAccessService {
	return &PrivilegedAccessService{repo: repo, audit: audit}
}

// Approve records a peer approval on a break-glass grant and audits it. The
// domain enforces the approval rules (distinct approver, not the requester,
// threshold → active); this persists the outcome and audits breakglass.approve,
// atomically.
func (s *PrivilegedAccessService) Approve(ctx context.Context, grantID, approverMembershipID uuid.UUID) (domain.PrivilegedGrant, error) {
	var out domain.PrivilegedGrant
	err := s.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		store := NewPrivilegedGrantStore(ttx)
		g, err := store.Get(ctx, grantID)
		if err != nil {
			return err
		}
		if err := g.Approve(approverMembershipID); err != nil {
			return err
		}
		if err := store.SaveDecision(ctx, g); err != nil {
			return err
		}
		if err := enqueueGrantProjection(ctx, ttx, g); err != nil {
			return err
		}
		out = g
		return emitAudit(ctx, ttx.Tx(), s.audit, g.OrganizationID, domain.ActionBreakglassApprove,
			domain.AuditTarget{Type: "privileged_grant", ID: g.ID.String(), Label: "aprovação de break-glass"},
			"aprovação de break-glass")
	})
	return out, err
}

// Revoke revokes an active grant, cascade-revokes its derived sessions and audits
// privileged.grant.revoke — atomically.
func (s *PrivilegedAccessService) Revoke(ctx context.Context, grantID uuid.UUID) error {
	return s.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		grantStore := NewPrivilegedGrantStore(ttx)
		g, err := grantStore.Get(ctx, grantID)
		if err != nil {
			return err
		}
		if err := g.Revoke(); err != nil {
			return err
		}
		if err := grantStore.SaveDecision(ctx, g); err != nil {
			return err
		}
		if err := enqueueGrantProjection(ctx, ttx, g); err != nil {
			return err
		}
		if _, err := NewTenantSessionStore(ttx).RevokeByGrant(ctx, g.ID); err != nil {
			return err
		}
		return emitAudit(ctx, ttx.Tx(), s.audit, g.OrganizationID, domain.ActionPrivilegedGrantRevoke,
			domain.AuditTarget{Type: "privileged_grant", ID: g.ID.String(), Label: "revogação de concessão"},
			"revogação de concessão privilegiada")
	})
}

// PassStepUp advances a just-requested break-glass grant past the reinforced-auth
// gate to awaiting_approval (or active when zero approvals are required), consuming
// the caller session's PROVEN, phishing-resistant AAL. In the /api/v1 flow this is
// the automatic continuation of the request — the L3 pipeline already performed the
// WebAuthn step-up — so it carries no separate audit beyond the request's; the
// resulting status lives on the grant. Refuses a non-phishing-resistant step-up
// (domain.ErrStepUpNotPhishingResistant: TOTP does not qualify for break-glass).
func (s *PrivilegedAccessService) PassStepUp(ctx context.Context, grantID uuid.UUID, provenAAL domain.AAL, phishingResistant bool) error {
	return s.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		store := NewPrivilegedGrantStore(ttx)
		g, err := store.Get(ctx, grantID)
		if err != nil {
			return err
		}
		if err := g.PassStepUp(provenAAL, phishingResistant); err != nil {
			return err
		}
		if err := store.SaveDecision(ctx, g); err != nil {
			return err
		}
		return enqueueGrantProjection(ctx, ttx, g)
	})
}

// RecordReview persists a post-use review and audits privileged.review —
// atomically.
func (s *PrivilegedAccessService) RecordReview(ctx context.Context, review domain.PostUseReview) error {
	return s.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		if err := NewPrivilegedGrantStore(ttx).RecordReview(ctx, review); err != nil {
			return err
		}
		return emitAudit(ctx, ttx.Tx(), s.audit, review.OrganizationID, domain.ActionPrivilegedReview,
			domain.AuditTarget{Type: "privileged_grant", ID: review.GrantID.String(), Label: "revisão pós-uso"},
			"revisão pós-uso de break-glass registrada")
	})
}
