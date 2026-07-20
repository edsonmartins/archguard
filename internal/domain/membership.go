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

// MembershipStatus is the lifecycle state of an identity's belonging to one
// organization. The machine is: invited → active → suspended ⇄ active, and any
// non-revoked state → revoked, which is terminal (re-adding a revoked member is
// a new membership, never a revival). The CHECK on the table mirrors Valid().
type MembershipStatus string

const (
	// MembershipInvited: created by an invitation (RFC-0002 §2.3), not yet
	// accepted; grants nothing until activated.
	MembershipInvited MembershipStatus = "invited"
	// MembershipActive: the member may operate in the organization with the roles
	// bound to this membership.
	MembershipActive MembershipStatus = "active"
	// MembershipSuspended: temporarily blocked in this organization; can resume.
	MembershipSuspended MembershipStatus = "suspended"
	// MembershipRevoked: terminal. Sessions in this tenant are ended (RFC-0002
	// R4); the row remains for the audit trail.
	MembershipRevoked MembershipStatus = "revoked"
)

// Valid reports whether s is a defined membership status.
func (s MembershipStatus) Valid() bool {
	switch s {
	case MembershipInvited, MembershipActive, MembershipSuspended, MembershipRevoked:
		return true
	default:
		return false
	}
}

// Errors returned by membership construction and lifecycle transitions.
var (
	// ErrInvalidMembership is returned when constructing a membership with a nil
	// identity or organization reference.
	ErrInvalidMembership = errors.New("membership: identidade e organização são obrigatórias")
	// ErrMembershipRevoked is returned when a transition is attempted on a revoked
	// membership — a terminal state (RFC-0002 R4).
	ErrMembershipRevoked = errors.New("membership: revogado é estado terminal")
	// ErrMembershipTransition is returned for a transition not allowed from the
	// current status (e.g. suspending an invited membership).
	ErrMembershipTransition = errors.New("membership: transição de estado inválida")
)

// Membership is the explicit identity↔organization relationship (RFC-0002 §2.3).
// Roles, permissions and tenant-specific attributes attach here, NEVER to the
// global identity (R2) — that is what keeps a role in organization A from
// leaking into organization B.
//
// Tenant-specific attributes are carried as ciphertext (AttributesEnc); the
// domain never sees plaintext. Timestamps of the lifecycle trail (ActivatedAt,
// RevokedAt) are stamped by the persistence layer when it writes the status
// change, so the domain stays free of the wall clock and deterministic in tests.
type Membership struct {
	ID             uuid.UUID
	IdentityID     uuid.UUID
	OrganizationID uuid.UUID
	Status         MembershipStatus
	// AttributesEnc is the tenant-scoped attribute ciphertext (matrícula, cost
	// center…). Isolated to the membership; never shared across tenants.
	AttributesEnc []byte
	// InvitedBy is the identity that issued the invitation, when the membership
	// began as an invite. Nil for a directly-created membership.
	InvitedBy *uuid.UUID
	// ActivatedAt / RevokedAt are set by the repository on the corresponding
	// transition; nil until then.
	ActivatedAt *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewMembership creates a directly-added, active membership — the common "add
// this identity to this organization" case (and the shape the data migration
// produces for existing users). Both references are required.
func NewMembership(identityID, organizationID uuid.UUID) (Membership, error) {
	m, err := newMembership(identityID, organizationID)
	if err != nil {
		return Membership{}, err
	}
	m.Status = MembershipActive
	return m, nil
}

// NewInvitedMembership creates a membership in the invited state, recording who
// issued the invitation. It grants nothing until Activate is called (the accept
// step of the invite flow, T-013). All three references are required.
func NewInvitedMembership(identityID, organizationID, invitedBy uuid.UUID) (Membership, error) {
	m, err := newMembership(identityID, organizationID)
	if err != nil {
		return Membership{}, err
	}
	if invitedBy == uuid.Nil {
		return Membership{}, fmt.Errorf("%w: invitedBy nulo", ErrInvalidMembership)
	}
	m.Status = MembershipInvited
	inv := invitedBy
	m.InvitedBy = &inv
	return m, nil
}

// newMembership mints the id and validates the required references, leaving the
// caller to set the initial status.
func newMembership(identityID, organizationID uuid.UUID) (Membership, error) {
	if identityID == uuid.Nil || organizationID == uuid.Nil {
		return Membership{}, ErrInvalidMembership
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Membership{}, fmt.Errorf("membership: geração de UUIDv7 falhou: %w", err)
	}
	return Membership{
		ID:             id,
		IdentityID:     identityID,
		OrganizationID: organizationID,
	}, nil
}

// Activate accepts an invitation: invited → active. It is only valid from the
// invited state.
func (m *Membership) Activate() error {
	return m.transition(MembershipActive, MembershipInvited)
}

// Suspend blocks the member in this organization: active → suspended.
func (m *Membership) Suspend() error {
	return m.transition(MembershipSuspended, MembershipActive)
}

// Resume returns a suspended membership to active: suspended → active.
func (m *Membership) Resume() error {
	return m.transition(MembershipActive, MembershipSuspended)
}

// Revoke terminates the membership from any non-revoked state. The caller ends
// the identity's sessions in this tenant (RFC-0002 R4). Idempotent: revoking an
// already-revoked membership is a no-op.
func (m *Membership) Revoke() {
	m.Status = MembershipRevoked
}

// transition moves to `to` only if the current status is one of `from`. A
// revoked membership rejects every transition (terminal, R4); any other
// disallowed source yields ErrMembershipTransition.
func (m *Membership) transition(to MembershipStatus, from ...MembershipStatus) error {
	if m.Status == MembershipRevoked {
		return ErrMembershipRevoked
	}
	for _, s := range from {
		if m.Status == s {
			m.Status = to
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", ErrMembershipTransition, m.Status, to)
}
