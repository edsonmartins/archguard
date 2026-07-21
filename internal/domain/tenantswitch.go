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
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Errors of the tenant-switch operation (T-012).
var (
	// ErrSameTenant is returned when the destination is already the active
	// tenant. That is not a switch: nothing changes, no token is reissued and no
	// switch event is audited.
	ErrSameTenant = errors.New("session: o destino já é o tenant ativo — nada a trocar")
	// ErrStepUpRequired is a DENIAL, not a failure: the destination tenant's
	// policy demands a stronger factor than the session has proven (ADR-0006:
	// "política mais restritiva vence"). The switch does not conclude until the
	// step-up (pacote 005) raises ProvenAAL.
	ErrStepUpRequired = errors.New("session: o tenant de destino exige fator mais forte — step-up necessário")
	// ErrDestinationPolicyUnavailable is returned when the destination tenant's
	// policy cannot be established (port error, undefined AAL). INV-6: a policy
	// that cannot decide denies — the switch never falls through open.
	ErrDestinationPolicyUnavailable = errors.New("session: política do tenant de destino indisponível — troca negada")
	// ErrSwitchAuditUnavailable is returned when the switch event could not be
	// recorded durably. I-5.4: the switch is then denied and rolled back — a
	// context change that cannot be audited does not happen.
	ErrSwitchAuditUnavailable = errors.New("session: auditoria da troca indisponível — troca negada")
)

// TenantSwitchEvent is the audit record of one tenant switch: which session of
// which identity moved from which membership/organization to which, under which
// proven AAL, and the token generation minted by the move. The tamper-evident
// trail is pacote 003; until then the SessionAuditor port carries it.
type TenantSwitchEvent struct {
	SessionID          uuid.UUID
	IdentityID         uuid.UUID
	FromMembershipID   uuid.UUID
	FromOrganizationID uuid.UUID
	ToMembershipID     uuid.UUID
	ToOrganizationID   uuid.UUID
	// ProvenAAL is the assurance level the session held when the switch was
	// allowed (already vetted against the destination's policy).
	ProvenAAL AAL
	// TokenGeneration is the NEW generation after the switch — always ≥ 2, since
	// a switch increments the session's counter.
	TokenGeneration int
}

// Validate refuses an event with missing references or an impossible
// generation, so a malformed record is never "audited" as if it were real.
func (e TenantSwitchEvent) Validate() error {
	for name, id := range map[string]uuid.UUID{
		"session": e.SessionID, "identity": e.IdentityID,
		"from_membership": e.FromMembershipID, "from_organization": e.FromOrganizationID,
		"to_membership": e.ToMembershipID, "to_organization": e.ToOrganizationID,
	} {
		if id == uuid.Nil {
			return fmt.Errorf("session: evento de troca sem %s", name)
		}
	}
	if !e.ProvenAAL.Valid() {
		return fmt.Errorf("session: evento de troca com AAL inválido %q", e.ProvenAAL)
	}
	if e.TokenGeneration < 2 {
		return fmt.Errorf("session: evento de troca com geração %d (troca sempre incrementa)", e.TokenGeneration)
	}
	return nil
}

// TenantAuthPolicy answers the authentication requirement of a tenant — the
// minimum AAL its policy demands for a session to operate in it. The real
// per-organization policy arrives with pacote 005 (MFA/step-up); this port is
// the seam. INV-6: an implementation that cannot decide MUST return an error
// (the caller denies), never a permissive default.
type TenantAuthPolicy interface {
	RequiredAAL(ctx context.Context, organizationID uuid.UUID) (AAL, error)
}

// SessionAuditor drains a tenant-switch event from the transactional outbox to
// the durable, tamper-evident trail (pacote 003). It runs ASYNCHRONOUSLY,
// OUTSIDE the switch transaction — so a remote call here is allowed (RFC-0004
// §4). The switch's own I-5.4 guarantee (audit-or-deny) is provided structurally
// by the outbox: the event row is written in the SAME transaction as the switch
// (migration 0016), so it commits or rolls back atomically with it. This port
// is the seam for that later drain, not something called inside the switch.
type SessionAuditor interface {
	RecordTenantSwitch(ctx context.Context, event TenantSwitchEvent) error
}

// SwitchTenant moves an ACTIVE session to another of the identity's tenants
// (design 002 §"Sessão e tenant ativo"): it validates the destination
// membership, enforces the destination's AAL requirement (step-up denial when
// stricter than proven — the step-up itself is pacote 005), moves the context
// and increments TokenGeneration so every token issued before the switch fails
// validation — the previous token is never reused, and the new token carries
// the destination's `org` alone (a token never spans two tenants).
//
// It returns the audit event of the move; the caller MUST write it to the
// transactional outbox within the same transaction that persists the switch,
// so the record is atomic with the switch (I-5.4: unrecorded switch = no
// switch). The durable trail drains that outbox out of band (pacote 003).
func (s *AuthSession) SwitchTenant(dest Membership, required AAL) (TenantSwitchEvent, error) {
	switch s.Status {
	case SessionRevoked:
		return TenantSwitchEvent{}, ErrSessionRevoked
	case SessionPendingSelection:
		return TenantSwitchEvent{}, ErrTenantSelectionRequired
	}
	if dest.IdentityID != s.IdentityID {
		return TenantSwitchEvent{}, fmt.Errorf("%w: membership %s é de %s", ErrForeignMembership, dest.ID, dest.IdentityID)
	}
	if dest.Status != MembershipActive {
		return TenantSwitchEvent{}, fmt.Errorf("%w: status %s", ErrMembershipNotSelectable, dest.Status)
	}
	if *s.MembershipID == dest.ID {
		return TenantSwitchEvent{}, ErrSameTenant
	}
	if !required.Valid() {
		return TenantSwitchEvent{}, fmt.Errorf("%w: AAL exigido %q indefinido", ErrDestinationPolicyUnavailable, required)
	}
	if !s.ProvenAAL.AtLeast(required) {
		return TenantSwitchEvent{}, fmt.Errorf("%w: comprovado %s, destino exige %s", ErrStepUpRequired, s.ProvenAAL, required)
	}

	ev := TenantSwitchEvent{
		SessionID:          s.ID,
		IdentityID:         s.IdentityID,
		FromMembershipID:   *s.MembershipID,
		FromOrganizationID: *s.OrganizationID,
		ToMembershipID:     dest.ID,
		ToOrganizationID:   dest.OrganizationID,
		ProvenAAL:          s.ProvenAAL,
	}
	s.setTenant(dest)
	s.TokenGeneration++
	ev.TokenGeneration = s.TokenGeneration
	return ev, nil
}

// TokenContext is the session-derived slice of the token claims contract
// (RFC-0006 §3): `sub` (opaque subject), `org` (active tenant — exactly one),
// `mid` (membership), `sid` (session), the proven AAL behind `acr`, and the
// token generation the mint must stamp. Serializing and signing the JWT is
// pacote 006; this gate is what forces the issuer to go through an ACTIVE
// session — there is no token for a pending or revoked one.
type TokenContext struct {
	Subject         string
	OrganizationID  uuid.UUID
	MembershipID    uuid.UUID
	SessionID       uuid.UUID
	ProvenAAL       AAL
	TokenGeneration int
}

// TokenContext derives the claims context of the session for the given
// identity (which supplies the opaque subject). It refuses an identity other
// than the session's, a pending session (ErrTenantSelectionRequired — the spec
// gate "no access token before explicit selection") and a revoked one.
func (s *AuthSession) TokenContext(idn Identity) (TokenContext, error) {
	if idn.ID != s.IdentityID {
		return TokenContext{}, fmt.Errorf("%w: identidade %s não é a da sessão", ErrInvalidSession, idn.ID)
	}
	mem, org, err := s.ActiveTenant()
	if err != nil {
		return TokenContext{}, err
	}
	return TokenContext{
		Subject:         idn.Subject,
		OrganizationID:  org,
		MembershipID:    mem,
		SessionID:       s.ID,
		ProvenAAL:       s.ProvenAAL,
		TokenGeneration: s.TokenGeneration,
	}, nil
}
