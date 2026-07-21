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

func TestAALAtLeast(t *testing.T) {
	cases := []struct {
		a, min AAL
		want   bool
	}{
		{AAL1, AAL1, true}, {AAL2, AAL1, true}, {AAL3, AAL1, true},
		{AAL1, AAL2, false}, {AAL2, AAL2, true}, {AAL3, AAL2, true},
		{AAL1, AAL3, false}, {AAL2, AAL3, false}, {AAL3, AAL3, true},
		// Fail-closed: nível indefinido não satisfaz nem é satisfeito.
		{AAL("aal9"), AAL1, false}, {AAL3, AAL("aal9"), false}, {"", AAL1, false},
	}
	for _, c := range cases {
		if got := c.a.AtLeast(c.min); got != c.want {
			t.Errorf("%q.AtLeast(%q) = %v, quero %v", c.a, c.min, got, c.want)
		}
	}
}

// activeSessionIn builds an identity with two active memberships and a session
// with tenant A already selected.
func activeSessionIn(t *testing.T, aal AAL) (AuthSession, Membership, Membership) {
	t.Helper()
	idn := uuid.New()
	a := mustMembership(t, idn)
	b := mustMembership(t, idn)
	s, err := NewAuthSession(idn, aal, []Membership{a, b})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := s.SelectTenant(a); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	return s, a, b
}

// Cenário "Troca de tenant": a troca move o contexto para o destino, incrementa
// a geração de token (o anterior não é reaproveitado) e produz o evento de
// auditoria com origem e destino.
func TestSwitchTenantMovesContextAndBumpsGeneration(t *testing.T) {
	s, a, b := activeSessionIn(t, AAL1)
	if s.TokenGeneration != 1 {
		t.Fatalf("geração inicial = %d, quero 1", s.TokenGeneration)
	}

	ev, err := s.SwitchTenant(b, AAL1)
	if err != nil {
		t.Fatalf("SwitchTenant: %v", err)
	}
	mem, org, err := s.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant: %v", err)
	}
	if mem != b.ID || org != b.OrganizationID {
		t.Fatalf("tenant ativo = (%s, %s), quero o destino (%s, %s)", mem, org, b.ID, b.OrganizationID)
	}
	if s.TokenGeneration != 2 {
		t.Fatalf("geração pós-troca = %d, quero 2 (token anterior invalidado)", s.TokenGeneration)
	}
	if ev.SessionID != s.ID || ev.IdentityID != s.IdentityID {
		t.Fatalf("evento com sessão/identidade errados: %+v", ev)
	}
	if ev.FromMembershipID != a.ID || ev.FromOrganizationID != a.OrganizationID {
		t.Fatalf("origem do evento errada: %+v", ev)
	}
	if ev.ToMembershipID != b.ID || ev.ToOrganizationID != b.OrganizationID {
		t.Fatalf("destino do evento errado: %+v", ev)
	}
	if ev.TokenGeneration != 2 {
		t.Fatalf("geração no evento = %d, quero 2", ev.TokenGeneration)
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("evento produzido deve ser válido: %v", err)
	}
}

// Cenário "Política mais restritiva no destino": destino exige fator mais forte
// que o comprovado ⇒ a troca NÃO conclui (step-up é o pacote 005) e nada muda.
func TestSwitchTenantStepUpRequired(t *testing.T) {
	s, a, b := activeSessionIn(t, AAL1)

	_, err := s.SwitchTenant(b, AAL2)
	if !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("destino mais forte: err = %v, quero ErrStepUpRequired", err)
	}
	// Estado intacto: tenant ainda A, geração ainda 1.
	mem, _, err := s.ActiveTenant()
	if err != nil || mem != a.ID {
		t.Fatalf("troca negada não pode mover o contexto: mem=%s err=%v", mem, err)
	}
	if s.TokenGeneration != 1 {
		t.Fatalf("troca negada não pode invalidar token: geração = %d", s.TokenGeneration)
	}

	// Comprovado igual ou mais forte que o exigido passa.
	s2, _, b2 := activeSessionIn(t, AAL2)
	if _, err := s2.SwitchTenant(b2, AAL2); err != nil {
		t.Fatalf("AAL2 comprovado vs AAL2 exigido deveria passar: %v", err)
	}
	s3, _, b3 := activeSessionIn(t, AAL3)
	if _, err := s3.SwitchTenant(b3, AAL2); err != nil {
		t.Fatalf("AAL3 comprovado vs AAL2 exigido deveria passar: %v", err)
	}
}

func TestSwitchTenantGuards(t *testing.T) {
	// Mesmo tenant: não é troca — recusa distinta, sem evento.
	s, a, _ := activeSessionIn(t, AAL1)
	if _, err := s.SwitchTenant(a, AAL1); !errors.Is(err, ErrSameTenant) {
		t.Fatalf("mesmo tenant: err = %v, quero ErrSameTenant", err)
	}

	// Membership de outra identidade.
	s, _, _ = activeSessionIn(t, AAL1)
	if _, err := s.SwitchTenant(mustMembership(t, uuid.New()), AAL1); !errors.Is(err, ErrForeignMembership) {
		t.Fatalf("membership alheio: err = %v, quero ErrForeignMembership", err)
	}

	// Destino não-ativo (suspenso).
	s, _, b := activeSessionIn(t, AAL1)
	if err := b.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if _, err := s.SwitchTenant(b, AAL1); !errors.Is(err, ErrMembershipNotSelectable) {
		t.Fatalf("destino suspenso: err = %v, quero ErrMembershipNotSelectable", err)
	}

	// Política devolveu AAL indefinido: fail-closed (INV-6), negação.
	s, _, b = activeSessionIn(t, AAL1)
	if _, err := s.SwitchTenant(b, AAL("aal9")); !errors.Is(err, ErrDestinationPolicyUnavailable) {
		t.Fatalf("AAL indefinido: err = %v, quero ErrDestinationPolicyUnavailable", err)
	}

	// Sessão pendente não troca — primeiro a seleção inicial.
	idn := uuid.New()
	m1, m2 := mustMembership(t, idn), mustMembership(t, idn)
	pending, err := NewAuthSession(idn, AAL1, []Membership{m1, m2})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if _, err := pending.SwitchTenant(m2, AAL1); !errors.Is(err, ErrTenantSelectionRequired) {
		t.Fatalf("troca em sessão pendente: err = %v, quero ErrTenantSelectionRequired", err)
	}

	// Sessão revogada é terminal.
	s, _, b = activeSessionIn(t, AAL1)
	s.Revoke()
	if _, err := s.SwitchTenant(b, AAL1); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("troca em sessão revogada: err = %v, quero ErrSessionRevoked", err)
	}
}

// O contexto de token só nasce de sessão ativa e carrega exatamente UM tenant
// (claims org/mid do RFC-0006 §3) e a geração corrente.
func TestTokenContext(t *testing.T) {
	idn, err := NewIdentity(IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	a := mustMembership(t, idn.ID)
	b := mustMembership(t, idn.ID)
	s, err := NewAuthSession(idn.ID, AAL2, []Membership{a, b})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}

	// Pendente: sem tenant, sem token.
	if _, err := s.TokenContext(idn); !errors.Is(err, ErrTenantSelectionRequired) {
		t.Fatalf("TokenContext pendente: err = %v, quero ErrTenantSelectionRequired", err)
	}

	if err := s.SelectTenant(a); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	before, err := s.TokenContext(idn)
	if err != nil {
		t.Fatalf("TokenContext: %v", err)
	}
	if before.Subject != idn.Subject {
		t.Fatalf("sub = %q, quero o subject opaco da identidade", before.Subject)
	}
	if before.OrganizationID != a.OrganizationID || before.MembershipID != a.ID {
		t.Fatalf("org/mid errados: %+v", before)
	}
	if before.SessionID != s.ID || before.ProvenAAL != AAL2 || before.TokenGeneration != 1 {
		t.Fatalf("sid/acr/geração errados: %+v", before)
	}

	// Troca: o novo contexto tem o org do destino e geração maior — um token
	// emitido antes (geração 1) nunca coincide com a geração corrente.
	if _, err := s.SwitchTenant(b, AAL1); err != nil {
		t.Fatalf("SwitchTenant: %v", err)
	}
	after, err := s.TokenContext(idn)
	if err != nil {
		t.Fatalf("TokenContext pós-troca: %v", err)
	}
	if after.OrganizationID != b.OrganizationID {
		t.Fatalf("org pós-troca = %s, quero %s", after.OrganizationID, b.OrganizationID)
	}
	if after.TokenGeneration <= before.TokenGeneration {
		t.Fatalf("geração deve crescer na troca: antes %d, depois %d",
			before.TokenGeneration, after.TokenGeneration)
	}

	// Identidade errada não gera contexto (sub de outra pessoa jamais).
	other, err := NewIdentity(IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if _, err := s.TokenContext(other); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("identidade alheia: err = %v, quero ErrInvalidSession", err)
	}

	// Revogada: sem token.
	s.Revoke()
	if _, err := s.TokenContext(idn); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("TokenContext revogada: err = %v, quero ErrSessionRevoked", err)
	}
}

func TestTenantSwitchEventValidate(t *testing.T) {
	s, _, b := activeSessionIn(t, AAL1)
	ev, err := s.SwitchTenant(b, AAL1)
	if err != nil {
		t.Fatalf("SwitchTenant: %v", err)
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("evento real deve validar: %v", err)
	}

	bad := ev
	bad.SessionID = uuid.Nil
	if err := bad.Validate(); err == nil {
		t.Fatalf("evento sem sessão deveria ser inválido")
	}
	bad = ev
	bad.ToMembershipID = uuid.Nil
	if err := bad.Validate(); err == nil {
		t.Fatalf("evento sem destino deveria ser inválido")
	}
	bad = ev
	bad.TokenGeneration = 1
	if err := bad.Validate(); err == nil {
		t.Fatalf("evento de troca com geração 1 deveria ser inválido (troca sempre incrementa)")
	}
}
