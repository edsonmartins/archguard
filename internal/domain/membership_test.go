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
	"testing"

	"github.com/google/uuid"
)

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}
	return id
}

func TestNewMembershipActiveByDefault(t *testing.T) {
	idn, org := mustUUID(t), mustUUID(t)
	m, err := NewMembership(idn, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	if m.Status != MembershipActive {
		t.Errorf("status = %q, quer active (adição direta)", m.Status)
	}
	if m.ID.Version() != 7 {
		t.Errorf("id deveria ser UUIDv7, veio versão %d", m.ID.Version())
	}
	if m.IdentityID != idn || m.OrganizationID != org {
		t.Error("referências não preservadas")
	}
	if m.InvitedBy != nil {
		t.Error("membership direto não tem invitedBy")
	}
}

func TestNewInvitedMembership(t *testing.T) {
	idn, org, inviter := mustUUID(t), mustUUID(t), mustUUID(t)
	m, err := NewInvitedMembership(idn, org, inviter)
	if err != nil {
		t.Fatalf("NewInvitedMembership: %v", err)
	}
	if m.Status != MembershipInvited {
		t.Errorf("status = %q, quer invited", m.Status)
	}
	if m.InvitedBy == nil || *m.InvitedBy != inviter {
		t.Errorf("invitedBy = %v, quer %v", m.InvitedBy, inviter)
	}
}

func TestNewMembershipRejectsNilRefs(t *testing.T) {
	valid := mustUUID(t)
	if _, err := NewMembership(uuid.Nil, valid); !errors.Is(err, ErrInvalidMembership) {
		t.Errorf("identity nula: erro = %v, quer ErrInvalidMembership", err)
	}
	if _, err := NewMembership(valid, uuid.Nil); !errors.Is(err, ErrInvalidMembership) {
		t.Errorf("org nula: erro = %v, quer ErrInvalidMembership", err)
	}
	if _, err := NewInvitedMembership(valid, valid, uuid.Nil); !errors.Is(err, ErrInvalidMembership) {
		t.Errorf("inviter nulo: erro = %v, quer ErrInvalidMembership", err)
	}
}

func TestMembershipInviteLifecycle(t *testing.T) {
	idn, org, inviter := mustUUID(t), mustUUID(t), mustUUID(t)
	m, _ := NewInvitedMembership(idn, org, inviter)

	if err := m.Activate(); err != nil {
		t.Fatalf("Activate (aceitar convite): %v", err)
	}
	if m.Status != MembershipActive {
		t.Fatalf("status = %q, quer active", m.Status)
	}
	if err := m.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if m.Status != MembershipSuspended {
		t.Fatalf("status = %q, quer suspended", m.Status)
	}
	if err := m.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if m.Status != MembershipActive {
		t.Fatalf("status = %q, quer active", m.Status)
	}
}

func TestMembershipInvalidTransitions(t *testing.T) {
	idn, org := mustUUID(t), mustUUID(t)

	// Suspend só é válido a partir de active.
	m, _ := NewInvitedMembership(idn, org, mustUUID(t))
	if err := m.Suspend(); !errors.Is(err, ErrMembershipTransition) {
		t.Errorf("Suspend de invited: erro = %v, quer ErrMembershipTransition", err)
	}
	// Resume só é válido a partir de suspended.
	active, _ := NewMembership(idn, org)
	if err := active.Resume(); !errors.Is(err, ErrMembershipTransition) {
		t.Errorf("Resume de active: erro = %v, quer ErrMembershipTransition", err)
	}
	// Activate só é válido a partir de invited.
	if err := active.Activate(); !errors.Is(err, ErrMembershipTransition) {
		t.Errorf("Activate de active: erro = %v, quer ErrMembershipTransition", err)
	}
}

func TestMembershipRevokeIsTerminal(t *testing.T) {
	idn, org := mustUUID(t), mustUUID(t)
	m, _ := NewMembership(idn, org)
	m.Revoke()
	if m.Status != MembershipRevoked {
		t.Fatalf("status = %q, quer revoked", m.Status)
	}
	// Nenhuma transição sai de revoked (R4).
	if err := m.Activate(); !errors.Is(err, ErrMembershipRevoked) {
		t.Errorf("Activate após revoke: erro = %v, quer ErrMembershipRevoked", err)
	}
	if err := m.Suspend(); !errors.Is(err, ErrMembershipRevoked) {
		t.Errorf("Suspend após revoke: erro = %v, quer ErrMembershipRevoked", err)
	}
	if err := m.Resume(); !errors.Is(err, ErrMembershipRevoked) {
		t.Errorf("Resume após revoke: erro = %v, quer ErrMembershipRevoked", err)
	}
	// Idempotente.
	m.Revoke()
	if m.Status != MembershipRevoked {
		t.Errorf("Revoke não idempotente: %q", m.Status)
	}
}

func TestMembershipStatusValid(t *testing.T) {
	for _, s := range []MembershipStatus{MembershipInvited, MembershipActive, MembershipSuspended, MembershipRevoked} {
		if !s.Valid() {
			t.Errorf("%q deveria ser válido", s)
		}
	}
	for _, s := range []MembershipStatus{"", "pending", "deleted"} {
		if s.Valid() {
			t.Errorf("%q não deveria ser válido", s)
		}
	}
}
