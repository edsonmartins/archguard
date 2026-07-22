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

// SessionStatus is the lifecycle state of an authenticated session's tenant
// context. The machine is: pending_selection → active → revoked, or
// pending_selection → revoked; revoked is terminal (a new login mints a new
// session, never a revival). The CHECK on the table mirrors Valid().
type SessionStatus string

const (
	// SessionPendingSelection: the identity authenticated but holds more than one
	// active membership, so the tenant must be chosen explicitly. No access token
	// is issued in this state (spec: "Múltiplos memberships no login").
	SessionPendingSelection SessionStatus = "pending_selection"
	// SessionActive: exactly one tenant is active; tokens carry its `org` claim.
	SessionActive SessionStatus = "active"
	// SessionRevoked: terminal. Set by logout or by the revocation cascade
	// (RFC-0002 R4, T-014); the row remains for the audit trail.
	SessionRevoked SessionStatus = "revoked"
)

// Valid reports whether s is a defined session status.
func (s SessionStatus) Valid() bool {
	switch s {
	case SessionPendingSelection, SessionActive, SessionRevoked:
		return true
	default:
		return false
	}
}

// Errors returned by session construction and lifecycle transitions.
var (
	// ErrInvalidSession is returned when constructing a session without an
	// identity or with an undefined proven AAL.
	ErrInvalidSession = errors.New("session: identidade e AAL comprovado são obrigatórios")
	// ErrNoActiveMembership is a DENIAL, not a failure: an identity with no
	// active membership does not get a session (ADR-0006 — there is no
	// "context-free" path into the product).
	ErrNoActiveMembership = errors.New("session: identidade sem membership ativo — autenticação negada")
	// ErrTenantSelectionRequired is returned while the session is pending: no
	// active tenant exists yet, so no access token may be issued.
	ErrTenantSelectionRequired = errors.New("session: seleção explícita de tenant é obrigatória antes da emissão de token")
	// ErrSessionRevoked is returned when operating on a revoked session — a
	// terminal state.
	ErrSessionRevoked = errors.New("session: revogada é estado terminal")
	// ErrSessionTransition is returned for a transition not allowed from the
	// current status (e.g. re-selecting the tenant of an active session — that is
	// the tenant SWITCH operation of T-012, with token reissue and audit).
	ErrSessionTransition = errors.New("session: transição de estado inválida")
	// ErrForeignMembership is returned when a membership of another identity is
	// offered as tenant context. Fail-closed: it is a caller bug, never filtered
	// silently.
	ErrForeignMembership = errors.New("session: membership não pertence à identidade da sessão")
	// ErrMembershipNotSelectable is returned when the offered membership is not
	// active (invited, suspended or revoked memberships grant no context).
	ErrMembershipNotSelectable = errors.New("session: apenas membership ativo pode ser tenant ativo")
)

// AuthSession is the tenant context of one authenticated session (design 002
// §"Sessão e tenant ativo"): it carries the identity and its ACTIVE membership.
// "Exactly one active tenant per session" is structural — there is a single
// MembershipID field, so two simultaneous tenants cannot be represented.
//
// MembershipID/OrganizationID are nil while the session is pending selection;
// both are set together when the tenant resolves. OrganizationID is the
// membership's organization, denormalized so tenant-scoped queries (Barreira 1,
// and RLS when enabled) can key on it; the composite FK in the table pins it to
// the membership so it cannot drift.
//
// Timestamps are stamped by the persistence layer, keeping the domain free of
// the wall clock and deterministic in tests (same rule as Membership).
type AuthSession struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	// MembershipID / OrganizationID form the active tenant context. Nil while
	// pending selection; set together on selection. They remain set after
	// revocation — the audit trail keeps which tenant the session was in.
	MembershipID   *uuid.UUID
	OrganizationID *uuid.UUID
	Status         SessionStatus
	// ProvenAAL is the authenticator assurance level demonstrated at login
	// (ADR-0010). The tenant switch (T-012) compares it against the destination
	// tenant's policy to demand step-up when the destination is stricter.
	ProvenAAL AAL
	// TokenGeneration is the token-invalidation counter of the session (T-012).
	// Every issued token carries the generation it was minted under; a tenant
	// switch increments it, so tokens from before the switch fail validation by
	// construction — "the previous token is never reused" (spec: "Troca de
	// tenant"). Starts at 1.
	TokenGeneration int
	// AuthTime is when the identity last AUTHENTICATED — set at login and bumped
	// on a step-up reauthentication (T-008/T-009). It is the OIDC `auth_time`
	// claim and the reference point for freshness policy. Distinct from CreatedAt:
	// the session may outlive several reauthentications. Stamped by the login flow
	// (the domain stays clock-free), so it is passed in, never read from now().
	AuthTime time.Time
	// AuthMethods are the factor TYPES demonstrated in the current authentication,
	// in the order proven. They are the source of the OIDC `amr` claim (AMR) and
	// the evidence that ProvenAAL is honest (SetAuthContext refuses an AAL above
	// what these methods can attest).
	AuthMethods []FactorType
	RevokedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewAuthSession resolves the tenant context of a fresh login from the
// identity's memberships (spec: "Contexto de tenant ativo na sessão"):
//
//   - no active membership → ErrNoActiveMembership (denial, no session);
//   - exactly one active membership → the session is born active with that
//     tenant auto-selected;
//   - more than one → the session is born pending_selection and no token can be
//     issued until SelectTenant (spec: "Múltiplos memberships no login").
//
// A membership belonging to another identity anywhere in the list is rejected
// (ErrForeignMembership) — fail-closed, never silently filtered.
func NewAuthSession(identityID uuid.UUID, provenAAL AAL, memberships []Membership) (AuthSession, error) {
	if identityID == uuid.Nil {
		return AuthSession{}, fmt.Errorf("%w: identidade nula", ErrInvalidSession)
	}
	if !provenAAL.Valid() {
		return AuthSession{}, fmt.Errorf("%w: AAL %q indefinido", ErrInvalidSession, provenAAL)
	}

	var active []Membership
	for _, m := range memberships {
		if m.IdentityID != identityID {
			return AuthSession{}, fmt.Errorf("%w: membership %s é de %s", ErrForeignMembership, m.ID, m.IdentityID)
		}
		if m.Status == MembershipActive {
			active = append(active, m)
		}
	}
	if len(active) == 0 {
		return AuthSession{}, ErrNoActiveMembership
	}

	id, err := uuid.NewV7()
	if err != nil {
		return AuthSession{}, fmt.Errorf("session: geração de UUIDv7 falhou: %w", err)
	}
	s := AuthSession{
		ID:              id,
		IdentityID:      identityID,
		ProvenAAL:       provenAAL,
		Status:          SessionPendingSelection,
		TokenGeneration: 1,
	}
	if len(active) == 1 {
		s.setTenant(active[0])
	}
	return s, nil
}

// ActiveTenant returns the active membership and organization of the session.
// It is the gate token issuance must pass: a pending session yields
// ErrTenantSelectionRequired and a revoked one ErrSessionRevoked — in neither
// case is there a tenant to put in the `org` claim.
func (s *AuthSession) ActiveTenant() (membershipID, organizationID uuid.UUID, err error) {
	switch s.Status {
	case SessionActive:
		// Belt-and-suspenders: an active session always carries both pointers
		// (setTenant sets them together), but never deref a nil — a malformed
		// row must deny, not panic (fail-closed).
		if s.MembershipID == nil || s.OrganizationID == nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("%w: sessão ativa sem tenant", ErrInvalidSession)
		}
		return *s.MembershipID, *s.OrganizationID, nil
	case SessionPendingSelection:
		return uuid.Nil, uuid.Nil, ErrTenantSelectionRequired
	case SessionRevoked:
		return uuid.Nil, uuid.Nil, ErrSessionRevoked
	default:
		// Zero-value or an unrecognized status (e.g. an unexpected value scanned
		// from the row): deny, never fall through to a nil deref.
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: status %q", ErrInvalidSession, s.Status)
	}
}

// SelectTenant resolves a pending session to the given membership — the
// explicit choice the spec demands when the identity has several. Only valid
// from pending_selection: an active session changes tenant via the switch
// operation (T-012), which reissues the token and audits the change.
func (s *AuthSession) SelectTenant(m Membership) error {
	switch s.Status {
	case SessionRevoked:
		return ErrSessionRevoked
	case SessionActive:
		return fmt.Errorf("%w: sessão já tem tenant ativo (troca é T-012)", ErrSessionTransition)
	}
	if m.IdentityID != s.IdentityID {
		return fmt.Errorf("%w: membership %s é de %s", ErrForeignMembership, m.ID, m.IdentityID)
	}
	if m.Status != MembershipActive {
		return fmt.Errorf("%w: status %s", ErrMembershipNotSelectable, m.Status)
	}
	s.setTenant(m)
	return nil
}

// Revoke terminates the session from any state. Idempotent. The tenant context
// is kept on the struct — the audit trail records where the session was.
func (s *AuthSession) Revoke() {
	s.Status = SessionRevoked
}

// setTenant pins the active tenant context and activates the session.
func (s *AuthSession) setTenant(m Membership) {
	mem, org := m.ID, m.OrganizationID
	s.MembershipID = &mem
	s.OrganizationID = &org
	s.Status = SessionActive
}

// SetAuthContext records the authentication facts of the current login — WHEN it
// happened (at, the OIDC auth_time) and WHICH factor types proved it (methods,
// the basis of amr). It is the honesty gate for acr: it REFUSES to record methods
// that cannot attest the session's ProvenAAL (e.g. a password-only login claiming
// AAL2), so the acr the session reports can never exceed the factors actually
// used. The login flow calls it right after NewAuthSession; step-up (T-009)
// re-records with the stronger method and a fresh auth_time.
func (s *AuthSession) SetAuthContext(at time.Time, methods []FactorType) error {
	if at.IsZero() {
		return fmt.Errorf("%w: auth_time ausente", ErrInvalidSession)
	}
	if len(methods) == 0 {
		return fmt.Errorf("%w: nenhum método de autenticação", ErrInvalidSession)
	}
	for _, m := range methods {
		if !m.Valid() {
			return fmt.Errorf("%w: método de autenticação %q inválido", ErrInvalidSession, m)
		}
	}
	// The proven AAL must be within reach of the strongest method used — a session
	// cannot claim more assurance than its factors can attest.
	if !strongestCeiling(methods).AtLeast(s.ProvenAAL) {
		return fmt.Errorf("%w: AAL comprovado %s excede o teto dos métodos usados", ErrInvalidSession, s.ProvenAAL)
	}
	s.AuthTime = at
	s.AuthMethods = append([]FactorType(nil), methods...)
	return nil
}

// strongestCeiling is the highest assurance any of the methods can attest — the
// max of each type's MaxAAL. Undefined when methods is empty.
func strongestCeiling(methods []FactorType) AAL {
	best := AAL("")
	bestRank := 0
	for _, m := range methods {
		if r := aalRank[MaxAAL(m)]; r > bestRank {
			bestRank, best = r, MaxAAL(m)
		}
	}
	return best
}

// ACR is the OIDC Authentication Context Class Reference of the session: the
// assurance level actually obtained, as its aal token ("aal1"/"aal2"/"aal3").
// Empty when the session carries no valid proven level (fail-closed — no acr is
// asserted rather than a false one).
func (s *AuthSession) ACR() string {
	if !s.ProvenAAL.Valid() {
		return ""
	}
	return string(s.ProvenAAL)
}

// PhishingResistant reports whether the session was authenticated with a
// phishing-resistant factor (WebAuthn) — the condition an L3 operation demands
// (OperationLevel.RequiresPhishingResistant). It reads the recorded AuthMethods,
// so a session that only ever proved a password/TOTP is not phishing-resistant
// even if some credential of the identity is.
func (s *AuthSession) PhishingResistant() bool {
	for _, ft := range s.AuthMethods {
		if ft == FactorWebAuthn {
			return true
		}
	}
	return false
}

// amrToken maps a factor type to its RFC 8176 Authentication Method Reference.
// Recovery codes have no standard token — a break-glass fallback is not
// advertised as a standing method — so they contribute nothing to amr.
func amrToken(t FactorType) (string, bool) {
	switch t {
	case FactorPassword:
		return "pwd", true
	case FactorTOTP:
		return "otp", true
	case FactorWebAuthn:
		return "hwk", true
	default: // recovery_code
		return "", false
	}
}

// AMR is the OIDC Authentication Methods References of the session (RFC 8176):
// the deduplicated method tokens for the factors used, in the order proven, plus
// "mfa" when two or more DISTINCT factor types authenticated the session. The
// output is deterministic for a given AuthMethods slice.
func (s *AuthSession) AMR() []string {
	out := make([]string, 0, len(s.AuthMethods)+1)
	seen := map[string]bool{}
	distinct := map[FactorType]bool{}
	for _, ft := range s.AuthMethods {
		distinct[ft] = true
		if tok, ok := amrToken(ft); ok && !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	if len(distinct) >= 2 {
		out = append(out, "mfa")
	}
	return out
}

// ErrNoIdentity is returned when an identity-scoped store is asked to operate
// without an identity context — the login-flow analogue of ErrNoTenant.
var ErrNoIdentity = errors.New("session: contexto de identidade é obrigatório")

// IdentityScope is the mandatory identity context of the session store used by
// the login flow (create session, select tenant, read own session). It mirrors
// TenantScope: there is no constructor without an identity, so the store's
// queries are always confined to one identity — Barreira 1 applied to the
// identity axis, since auth_session rows pending selection have no tenant yet.
type IdentityScope struct {
	identityID uuid.UUID
}

// NewIdentityScope builds a scope for identityID, refusing uuid.Nil.
func NewIdentityScope(identityID uuid.UUID) (IdentityScope, error) {
	if identityID == uuid.Nil {
		return IdentityScope{}, ErrNoIdentity
	}
	return IdentityScope{identityID: identityID}, nil
}

// IdentityID returns the identity this scope is bound to.
func (s IdentityScope) IdentityID() uuid.UUID {
	return s.identityID
}
