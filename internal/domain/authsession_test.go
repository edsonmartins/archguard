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

// mustMembership builds an active membership for identityID in a fresh org.
func mustMembership(t *testing.T, identityID uuid.UUID) Membership {
	t.Helper()
	m, err := NewMembership(identityID, uuid.New())
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	return m
}

func TestNewAuthSessionRejectsInvalidInput(t *testing.T) {
	idn := uuid.New()
	if _, err := NewAuthSession(uuid.Nil, AAL1, nil); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("identidade nula: err = %v, quero ErrInvalidSession", err)
	}
	if _, err := NewAuthSession(idn, AAL("aal9"), nil); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("AAL inválido: err = %v, quero ErrInvalidSession", err)
	}
}

// Scenario base: autenticação sem membership ativo é negada — não nasce sessão.
func TestNewAuthSessionDeniesWithoutActiveMembership(t *testing.T) {
	idn := uuid.New()

	if _, err := NewAuthSession(idn, AAL1, nil); !errors.Is(err, ErrNoActiveMembership) {
		t.Fatalf("sem memberships: err = %v, quero ErrNoActiveMembership", err)
	}

	// Memberships existem, mas nenhum em estado active: invited, suspended, revoked.
	invited, err := NewInvitedMembership(idn, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("NewInvitedMembership: %v", err)
	}
	suspended := mustMembership(t, idn)
	if err := suspended.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	revoked := mustMembership(t, idn)
	revoked.Revoke()

	_, err = NewAuthSession(idn, AAL1, []Membership{invited, suspended, revoked})
	if !errors.Is(err, ErrNoActiveMembership) {
		t.Fatalf("só memberships não-ativos: err = %v, quero ErrNoActiveMembership", err)
	}
}

// Fail-closed: um membership de OUTRA identidade na lista é um bug do chamador,
// não algo a filtrar em silêncio.
func TestNewAuthSessionRejectsForeignMembership(t *testing.T) {
	idn := uuid.New()
	foreign := mustMembership(t, uuid.New())

	_, err := NewAuthSession(idn, AAL1, []Membership{foreign})
	if !errors.Is(err, ErrForeignMembership) {
		t.Fatalf("membership alheio: err = %v, quero ErrForeignMembership", err)
	}
}

// Um único membership ativo: o tenant é resolvido sem interação — a sessão já
// nasce com contexto ativo.
func TestNewAuthSessionAutoSelectsSingleMembership(t *testing.T) {
	idn := uuid.New()
	m := mustMembership(t, idn)

	s, err := NewAuthSession(idn, AAL2, []Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if s.Status != SessionActive {
		t.Fatalf("status = %s, quero active", s.Status)
	}
	if s.ProvenAAL != AAL2 {
		t.Fatalf("ProvenAAL = %s, quero aal2", s.ProvenAAL)
	}
	mem, org, err := s.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant: %v", err)
	}
	if mem != m.ID || org != m.OrganizationID {
		t.Fatalf("tenant ativo = (%s, %s), quero (%s, %s)", mem, org, m.ID, m.OrganizationID)
	}
	if s.ID.Version() != 7 {
		t.Fatalf("id não é UUIDv7: %s", s.ID)
	}
}

// Cenário "Múltiplos memberships no login": com mais de um membership ativo a
// sessão nasce pendente e NÃO expõe tenant ativo — a seleção explícita é
// obrigatória antes de emitir token de acesso.
func TestNewAuthSessionRequiresExplicitSelection(t *testing.T) {
	idn := uuid.New()
	a := mustMembership(t, idn)
	b := mustMembership(t, idn)

	s, err := NewAuthSession(idn, AAL1, []Membership{a, b})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if s.Status != SessionPendingSelection {
		t.Fatalf("status = %s, quero pending_selection", s.Status)
	}
	if s.MembershipID != nil || s.OrganizationID != nil {
		t.Fatalf("sessão pendente não pode carregar tenant: %v/%v", s.MembershipID, s.OrganizationID)
	}
	if _, _, err := s.ActiveTenant(); !errors.Is(err, ErrTenantSelectionRequired) {
		t.Fatalf("ActiveTenant pendente: err = %v, quero ErrTenantSelectionRequired", err)
	}

	// Seleção explícita resolve o contexto: exatamente um tenant ativo.
	if err := s.SelectTenant(b); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	mem, org, err := s.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant pós-seleção: %v", err)
	}
	if mem != b.ID || org != b.OrganizationID {
		t.Fatalf("tenant ativo = (%s, %s), quero o membership selecionado (%s, %s)",
			mem, org, b.ID, b.OrganizationID)
	}
}

func TestSelectTenantGuards(t *testing.T) {
	idn := uuid.New()
	a := mustMembership(t, idn)
	b := mustMembership(t, idn)

	pending := func() AuthSession {
		s, err := NewAuthSession(idn, AAL1, []Membership{a, b})
		if err != nil {
			t.Fatalf("NewAuthSession: %v", err)
		}
		return s
	}

	// Membership de outra identidade nunca vira tenant ativo.
	s := pending()
	if err := s.SelectTenant(mustMembership(t, uuid.New())); !errors.Is(err, ErrForeignMembership) {
		t.Fatalf("membership alheio: err = %v, quero ErrForeignMembership", err)
	}

	// Só membership ATIVO é selecionável — suspenso/convidado/revogado não.
	s = pending()
	sus := mustMembership(t, idn)
	if err := sus.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if err := s.SelectTenant(sus); !errors.Is(err, ErrMembershipNotSelectable) {
		t.Fatalf("membership suspenso: err = %v, quero ErrMembershipNotSelectable", err)
	}

	// Sessão já ativa não re-seleciona: troca de tenant é operação distinta,
	// com reemissão de token e auditoria (T-012).
	s = pending()
	if err := s.SelectTenant(a); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	if err := s.SelectTenant(b); !errors.Is(err, ErrSessionTransition) {
		t.Fatalf("re-seleção em sessão ativa: err = %v, quero ErrSessionTransition", err)
	}

	// Sessão revogada é terminal.
	s = pending()
	s.Revoke()
	if err := s.SelectTenant(a); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("seleção em sessão revogada: err = %v, quero ErrSessionRevoked", err)
	}
}

func TestRevokeIsTerminalAndIdempotent(t *testing.T) {
	idn := uuid.New()
	m := mustMembership(t, idn)
	s, err := NewAuthSession(idn, AAL1, []Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}

	s.Revoke()
	s.Revoke() // idempotente
	if s.Status != SessionRevoked {
		t.Fatalf("status = %s, quero revoked", s.Status)
	}
	if _, _, err := s.ActiveTenant(); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("ActiveTenant revogada: err = %v, quero ErrSessionRevoked", err)
	}
	// O tenant que a sessão tinha permanece registrado (trilha), mesmo revogada.
	if s.MembershipID == nil || *s.MembershipID != m.ID {
		t.Fatalf("membership da sessão revogada deve permanecer para a trilha")
	}
}

// Fail-closed: a malformed session (zero value, or an unrecognized status
// string) must DENY, never panic on the nil tenant pointers.
func TestActiveTenantFailsClosedOnMalformedSession(t *testing.T) {
	// Zero-value session: Status == "", both pointers nil.
	var zero AuthSession
	if _, _, err := zero.ActiveTenant(); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("sessão zero: err = %v, quero ErrInvalidSession (sem panic)", err)
	}

	// Unrecognized status string (e.g. scanned from an unexpected row value).
	weird := AuthSession{Status: SessionStatus("bogus")}
	if _, _, err := weird.ActiveTenant(); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("status desconhecido: err = %v, quero ErrInvalidSession", err)
	}

	// Status active but pointers nil (impossible via constructors, but must not
	// panic if ever constructed by hand or scanned malformed).
	active := AuthSession{Status: SessionActive}
	if _, _, err := active.ActiveTenant(); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("active sem tenant: err = %v, quero ErrInvalidSession", err)
	}
}

func TestIdentityScopeRefusesNil(t *testing.T) {
	if _, err := NewIdentityScope(uuid.Nil); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("escopo nulo: err = %v, quero ErrNoIdentity", err)
	}
	id := uuid.New()
	sc, err := NewIdentityScope(id)
	if err != nil {
		t.Fatalf("NewIdentityScope: %v", err)
	}
	if sc.IdentityID() != id {
		t.Fatalf("IdentityID = %s, quero %s", sc.IdentityID(), id)
	}
}
