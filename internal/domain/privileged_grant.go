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

package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GrantOrigin records how a privileged grant came to be: a NORMAL time-limited
// grant an admin issues, or an emergency BREAKGLASS grant (ADR-0008). The origin
// travels with the grant so the audit trail and the PDP (pacote 007) can tell an
// emergency access apart from routine delegation.
type GrantOrigin string

const (
	GrantNormal     GrantOrigin = "normal"
	GrantBreakglass GrantOrigin = "breakglass"
)

// Valid reports whether o is a defined origin.
func (o GrantOrigin) Valid() bool {
	return o == GrantNormal || o == GrantBreakglass
}

// GrantStatus is the lifecycle state of a privileged grant (design 004
// §"Break-glass"). The machine is: requested → awaiting_approval → active →
// (expired | revoked); and requested → denied, awaiting_approval → (rejected |
// expired). active/expired/revoked/denied/rejected are terminal for new
// approvals. The transitions are the break-glass state machine (T-007); this
// type carries the state and the decision-time evaluation.
type GrantStatus string

const (
	GrantRequested        GrantStatus = "requested"
	GrantAwaitingApproval GrantStatus = "awaiting_approval"
	GrantActive           GrantStatus = "active"
	GrantExpired          GrantStatus = "expired"
	GrantRevoked          GrantStatus = "revoked"
	GrantDenied           GrantStatus = "denied"
	GrantRejected         GrantStatus = "rejected"
)

// Valid reports whether s is a defined status.
func (s GrantStatus) Valid() bool {
	switch s {
	case GrantRequested, GrantAwaitingApproval, GrantActive, GrantExpired,
		GrantRevoked, GrantDenied, GrantRejected:
		return true
	default:
		return false
	}
}

// GrantTarget is what a privileged grant authorizes access to: an opaque asset
// reference plus the scope of privilege over it (design 004 §"Concessões":
// "alvo (ativo/escopo)"). Type/ID are opaque identifiers — the actual asset
// catalog belongs to the access-brokering components, not to ArchGuard — and
// Scope names the privilege (e.g. "read", "admin").
type GrantTarget struct {
	Type  string
	ID    string
	Scope string
}

// Valid reports whether the target is fully specified.
func (t GrantTarget) Valid() bool {
	return t.Type != "" && t.ID != "" && t.Scope != ""
}

// GrantApproval is one peer approval recorded on a grant.
type GrantApproval struct {
	ApproverMembershipID uuid.UUID
}

// Errors of privileged-grant construction and transitions.
var (
	ErrInvalidGrant = errors.New("privileged_grant: dados obrigatórios ausentes")
	// ErrInvalidGrantWindow is returned when the window is not a positive interval
	// (expiry must be strictly after the start).
	ErrInvalidGrantWindow = errors.New("privileged_grant: janela temporal inválida")
	// ErrGrantTransition is returned for a state transition not allowed from the
	// current status.
	ErrGrantTransition = errors.New("privileged_grant: transição de estado inválida")
	// ErrGrantDuplicateApproval is returned when the same approver approves twice —
	// the threshold must be met by DISTINCT approvers.
	ErrGrantDuplicateApproval = errors.New("privileged_grant: aprovação duplicada do mesmo par")
	// ErrStepUpNotPhishingResistant is returned when the break-glass step-up was
	// completed with a factor that is not phishing-resistant (e.g. TOTP): emergency
	// access demands WebAuthn (ADR-0008 / spec "Fator insuficiente").
	ErrStepUpNotPhishingResistant = errors.New("privileged_grant: break-glass exige step-up com fator resistente a phishing (WebAuthn)")
)

// PrivilegedGrant is a time-limited authorization for a membership to act on a
// target with a given scope (design 004 §"Concessões"). It is the record the
// break-glass workflow produces and the PDP (pacote 007) reads as a relation.
// Its defining safety property is that AUTHORITY IS EVALUATED AT DECISION TIME,
// not merely by a cleanup job: a grant whose window has passed authorizes
// nothing even if its status column still says active and a token minted under
// it is presented (spec "Token emitido antes da expiração").
type PrivilegedGrant struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	// SubjectMembershipID is who holds the privilege — a MEMBERSHIP, never the
	// global identity (R2): privilege is always held in one tenant's context and
	// never follows the person into another organization.
	SubjectMembershipID uuid.UUID
	Target              GrantTarget
	Origin              GrantOrigin
	Status              GrantStatus
	RequiredApprovals   int
	Approvals           []GrantApproval
	// NotBefore/ExpiresAt bound the grant's validity window. Authority holds only
	// strictly within [NotBefore, ExpiresAt).
	NotBefore time.Time
	ExpiresAt time.Time
	// Justification / IncidentRef are MANDATORY for a break-glass grant (T-008):
	// the reason for the emergency access and the incident/ticket it is bound to.
	// They are empty for a normal grant. Justification is operator-supplied text
	// (may carry contextual personal data — treated as tenant content, not indexed
	// by person); IncidentRef is an opaque ticket reference.
	Justification string
	IncidentRef   string
}

// NewPrivilegedGrant builds a grant in the requested state. It validates the
// references, the target and a POSITIVE window; the approval threshold and the
// origin-specific rules (justification, step-up, non-zero approvers in
// production) are enforced by the break-glass request flow (T-007/T-008/T-010).
// Timestamps are supplied by the caller (a trusted clock), keeping the domain
// free of the wall clock.
func NewPrivilegedGrant(organizationID, subjectMembershipID uuid.UUID, target GrantTarget, origin GrantOrigin, requiredApprovals int, notBefore, expiresAt time.Time) (PrivilegedGrant, error) {
	if organizationID == uuid.Nil || subjectMembershipID == uuid.Nil {
		return PrivilegedGrant{}, fmt.Errorf("%w: organização/subject", ErrInvalidGrant)
	}
	if !target.Valid() {
		return PrivilegedGrant{}, fmt.Errorf("%w: alvo incompleto", ErrInvalidGrant)
	}
	if !origin.Valid() {
		return PrivilegedGrant{}, fmt.Errorf("%w: origem %q", ErrInvalidGrant, origin)
	}
	if requiredApprovals < 0 {
		return PrivilegedGrant{}, fmt.Errorf("%w: aprovações exigidas negativas", ErrInvalidGrant)
	}
	if notBefore.IsZero() || expiresAt.IsZero() || !expiresAt.After(notBefore) {
		return PrivilegedGrant{}, ErrInvalidGrantWindow
	}
	id, err := uuid.NewV7()
	if err != nil {
		return PrivilegedGrant{}, fmt.Errorf("privileged_grant: geração de UUIDv7 falhou: %w", err)
	}
	return PrivilegedGrant{
		ID:                  id,
		OrganizationID:      organizationID,
		SubjectMembershipID: subjectMembershipID,
		Target:              target,
		Origin:              origin,
		Status:              GrantRequested,
		RequiredApprovals:   requiredApprovals,
		NotBefore:           notBefore,
		ExpiresAt:           expiresAt,
	}, nil
}

// PassStepUp advances a requested grant past the reinforced-authentication gate
// to awaiting_approval — the "solicitado ──step-up OK──► aguardando_aprovacao"
// edge (design 004). It consumes the step-up result from pacote 005 (the
// session's proven AAL and whether it was phishing-resistant) and REFUSES a
// step-up that is not phishing-resistant: TOTP does not qualify for break-glass,
// only WebAuthn (ErrStepUpNotPhishingResistant / spec "Fator insuficiente").
// When zero approvals are required (a dev-only configuration; production forbids
// it, T-010) the grant becomes active at once.
func (g *PrivilegedGrant) PassStepUp(provenAAL AAL, phishingResistant bool) error {
	if g.Status != GrantRequested {
		return fmt.Errorf("%w: step-up exige status requested, está %s", ErrGrantTransition, g.Status)
	}
	if !phishingResistant || !provenAAL.AtLeast(AAL2) {
		return ErrStepUpNotPhishingResistant
	}
	if g.RequiredApprovals == 0 {
		g.Status = GrantActive
	} else {
		g.Status = GrantAwaitingApproval
	}
	return nil
}

// Deny terminates a requested grant that did not pass the gate (e.g. step-up
// failed) — the "solicitado ─ negado" edge. Valid only from requested.
func (g *PrivilegedGrant) Deny() error {
	if g.Status != GrantRequested {
		return fmt.Errorf("%w: negação exige status requested, está %s", ErrGrantTransition, g.Status)
	}
	g.Status = GrantDenied
	return nil
}

// Approve records one peer approval. The threshold must be met by DISTINCT
// approvers (a duplicate is ErrGrantDuplicateApproval); when the count reaches
// RequiredApprovals the grant becomes active. Valid only from awaiting_approval.
// The rules that an approver is neither the requester nor a delegation session
// are enforced by the request flow (T-010).
func (g *PrivilegedGrant) Approve(approverMembershipID uuid.UUID) error {
	if g.Status != GrantAwaitingApproval {
		return fmt.Errorf("%w: aprovação exige status awaiting_approval, está %s", ErrGrantTransition, g.Status)
	}
	if approverMembershipID == uuid.Nil {
		return fmt.Errorf("%w: aprovador nulo", ErrInvalidGrant)
	}
	for _, a := range g.Approvals {
		if a.ApproverMembershipID == approverMembershipID {
			return ErrGrantDuplicateApproval
		}
	}
	g.Approvals = append(g.Approvals, GrantApproval{ApproverMembershipID: approverMembershipID})
	if len(g.Approvals) >= g.RequiredApprovals {
		g.Status = GrantActive
	}
	return nil
}

// Reject terminates a grant awaiting approval — the "aguardando_aprovacao ─
// rejeitado" edge. Valid only from awaiting_approval.
func (g *PrivilegedGrant) Reject() error {
	if g.Status != GrantAwaitingApproval {
		return fmt.Errorf("%w: rejeição exige status awaiting_approval, está %s", ErrGrantTransition, g.Status)
	}
	g.Status = GrantRejected
	return nil
}

// Expire moves a grant to expired when its window has passed at now. Valid from
// awaiting_approval or active (both edges lead to expired). It refuses to expire
// a grant whose window has NOT passed (fail-safe: no premature expiry). This is
// the state materialization the cleanup job performs (T-012); Authorizes already
// denies a past-window grant regardless of whether this ran.
func (g *PrivilegedGrant) Expire(now time.Time) error {
	if g.Status != GrantAwaitingApproval && g.Status != GrantActive {
		return fmt.Errorf("%w: expiração exige awaiting_approval ou active, está %s", ErrGrantTransition, g.Status)
	}
	if !g.Expired(now) {
		return fmt.Errorf("%w: janela ainda não expirou", ErrGrantTransition)
	}
	g.Status = GrantExpired
	return nil
}

// Revoke terminates an active grant immediately — the "ativo ─ revogado" edge and
// the trigger for the cascade revocation of derived sessions (T-012). Valid only
// from active.
func (g *PrivilegedGrant) Revoke() error {
	if g.Status != GrantActive {
		return fmt.Errorf("%w: revogação exige status active, está %s", ErrGrantTransition, g.Status)
	}
	g.Status = GrantRevoked
	return nil
}

// NewBreakglassRequest opens an emergency-access grant (origin breakglass). Over
// NewPrivilegedGrant it additionally REQUIRES a justification and an incident
// reference (T-008 / spec "justificativa vinculada a incidente"): an emergency
// access with no stated reason and no incident to bind it to is refused. The
// subject membership is the requesting operator (who receives the access); the
// approval threshold, step-up factor and non-zero-in-production rules are checked
// by the request flow (T-009/T-010).
func NewBreakglassRequest(organizationID, subjectMembershipID uuid.UUID, target GrantTarget, requiredApprovals int, justification, incidentRef string, notBefore, expiresAt time.Time) (PrivilegedGrant, error) {
	if justification == "" {
		return PrivilegedGrant{}, fmt.Errorf("%w: justificativa obrigatória no break-glass", ErrInvalidGrant)
	}
	if incidentRef == "" {
		return PrivilegedGrant{}, fmt.Errorf("%w: referência de incidente obrigatória no break-glass", ErrInvalidGrant)
	}
	g, err := NewPrivilegedGrant(organizationID, subjectMembershipID, target, GrantBreakglass, requiredApprovals, notBefore, expiresAt)
	if err != nil {
		return PrivilegedGrant{}, err
	}
	g.Justification = justification
	g.IncidentRef = incidentRef
	return g, nil
}

// Expired reports whether the grant's window has passed at now — independent of
// the status column, so a not-yet-materialized expiry is still recognized.
func (g PrivilegedGrant) Expired(now time.Time) bool {
	return !now.Before(g.ExpiresAt)
}

// Authorizes reports whether the grant CURRENTLY confers access at now: it must
// be active AND within its window. This is the decision-time check every access
// decision makes — a grant past its window authorizes nothing even if its status
// was never updated to expired (spec "Token emitido antes da expiração"). It is
// fail-closed: any status other than active, or a now outside the window, denies.
func (g PrivilegedGrant) Authorizes(now time.Time) bool {
	if g.Status != GrantActive {
		return false
	}
	if now.Before(g.NotBefore) || g.Expired(now) {
		return false
	}
	return true
}
