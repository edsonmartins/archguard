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

// DelegationStatus is the lifecycle of a delegation (impersonation) session
// (ADR-0008 §2). Consent is the default, so a delegation is born awaiting the
// target's consent; it becomes active only once consented, and is terminal on
// revoke/expire/deny. There is no path that starts active without consent —
// non-consented access exists ONLY through break-glass (a separate flow).
type DelegationStatus string

const (
	DelegationPendingConsent DelegationStatus = "pending_consent"
	DelegationActive         DelegationStatus = "active"
	DelegationRevoked        DelegationStatus = "revoked"
	DelegationExpired        DelegationStatus = "expired"
	DelegationDenied         DelegationStatus = "denied"
)

// Valid reports whether s is a defined delegation status.
func (s DelegationStatus) Valid() bool {
	switch s {
	case DelegationPendingConsent, DelegationActive, DelegationRevoked, DelegationExpired, DelegationDenied:
		return true
	default:
		return false
	}
}

// ActClaim is the RFC 8693 `act` claim — the ACTUAL party acting. Sub is its
// opaque subject; Act nests a further actor for a delegation lineage (token
// exchange). It never carries a name or e-mail — only the opaque subject, so the
// token leaks no personal data.
type ActClaim struct {
	Sub string    `json:"sub"`
	Act *ActClaim `json:"act,omitempty"`
}

// DelegationTokenClaims are the identity claims of a delegation token (ADR-0008
// §2 / RFC 8693). Sub is the IMPERSONATED subject (the identity the action runs
// as); Act is the REAL actor. Delegated is always true — it marks the token so
// downstream (the session banner T-005, the scope guard T-003) treats it as a
// delegation and never as an ordinary session. The signed JWT that carries these
// claims is minted by the OIDC layer (pacote 006); this is the domain content.
type DelegationTokenClaims struct {
	Sub            string   `json:"sub"`
	Act            ActClaim `json:"act"`
	OrganizationID string   `json:"org"`
	ExpiresAt      int64    `json:"exp"`
	Delegated      bool     `json:"delegated"`
}

// Errors of delegation construction and token emission.
var (
	ErrInvalidDelegation = errors.New("delegation: dados obrigatórios ausentes")
	// ErrDelegationNotActive is returned when emitting a token from a delegation
	// that is not active, or whose window has passed — fail-closed: an expired or
	// unconsented delegation mints no token.
	ErrDelegationNotActive = errors.New("delegation: apenas delegação ativa e vigente emite token")
)

// Delegation is a consented, time-limited impersonation of a target identity by
// a real actor (ADR-0008 §2). Its safety properties are structural: the real
// actor is always recorded (RealActorSubject → the act claim), the window is
// short and explicit, and a token is emitted only while active AND within the
// window.
type Delegation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	// RealActorMembershipID / RealActorSubject identify the operator performing
	// the delegation. The subject is opaque (non-personal) and becomes the act
	// claim's sub.
	RealActorMembershipID uuid.UUID
	RealActorSubject      string
	// TargetIdentityID / TargetSubject identify the impersonated identity; the
	// subject becomes the token's sub.
	TargetIdentityID uuid.UUID
	TargetSubject    string
	Status           DelegationStatus
	NotBefore        time.Time
	ExpiresAt        time.Time
}

// NewDelegation opens a delegation in the pending_consent state. It validates the
// references and a POSITIVE window. Consent (T-004) moves it to active; there is
// no constructor that starts active. The subjects are the opaque identity
// subjects (never e-mail/name).
func NewDelegation(organizationID, realActorMembershipID uuid.UUID, realActorSubject string, targetIdentityID uuid.UUID, targetSubject string, notBefore, expiresAt time.Time) (Delegation, error) {
	if organizationID == uuid.Nil || realActorMembershipID == uuid.Nil || targetIdentityID == uuid.Nil {
		return Delegation{}, fmt.Errorf("%w: referências", ErrInvalidDelegation)
	}
	if realActorSubject == "" || targetSubject == "" {
		return Delegation{}, fmt.Errorf("%w: subjects", ErrInvalidDelegation)
	}
	if realActorSubject == targetSubject {
		return Delegation{}, fmt.Errorf("%w: ator real e alvo não podem ser o mesmo sujeito", ErrInvalidDelegation)
	}
	if notBefore.IsZero() || expiresAt.IsZero() || !expiresAt.After(notBefore) {
		return Delegation{}, fmt.Errorf("%w: janela temporal inválida", ErrInvalidDelegation)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Delegation{}, fmt.Errorf("delegation: geração de UUIDv7 falhou: %w", err)
	}
	return Delegation{
		ID:                    id,
		OrganizationID:        organizationID,
		RealActorMembershipID: realActorMembershipID,
		RealActorSubject:      realActorSubject,
		TargetIdentityID:      targetIdentityID,
		TargetSubject:         targetSubject,
		Status:                DelegationPendingConsent,
		NotBefore:             notBefore,
		ExpiresAt:             expiresAt,
	}, nil
}

// Active reports whether the delegation currently confers impersonation at now:
// status active AND within the window. Fail-closed on any other status or a now
// outside the window.
func (d Delegation) Active(now time.Time) bool {
	return d.Status == DelegationActive && !now.Before(d.NotBefore) && now.Before(d.ExpiresAt)
}

// TokenClaims emits the delegation token's identity claims at now: sub = the
// impersonated subject, act = the real actor. It refuses to emit unless the
// delegation is active and within its window (ErrDelegationNotActive) — an
// expired or unconsented delegation mints no token (fail-closed).
func (d Delegation) TokenClaims(now time.Time) (DelegationTokenClaims, error) {
	if !d.Active(now) {
		return DelegationTokenClaims{}, fmt.Errorf("%w: status %s", ErrDelegationNotActive, d.Status)
	}
	return DelegationTokenClaims{
		Sub:            d.TargetSubject,
		Act:            ActClaim{Sub: d.RealActorSubject},
		OrganizationID: d.OrganizationID.String(),
		ExpiresAt:      d.ExpiresAt.Unix(),
		Delegated:      true,
	}, nil
}

// AuditActor builds the actor for an audit event performed under this delegation:
// the apparent subject is the impersonated identity, and Act names the REAL
// actor — so every delegated action records BOTH and the trail can reconstruct
// who really executed it (spec "cada evento de auditoria registra ambos").
func (d Delegation) AuditActor() AuditActor {
	return AuditActor{
		IdentitySubject: d.TargetSubject,
		Act:             &AuditActor{IdentitySubject: d.RealActorSubject},
	}
}
