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
	"time"
)

// Cada nível mapeia para o AAL e a resistência a phishing do ADR-0010.
func TestAssuranceLevelRequirements(t *testing.T) {
	cases := []struct {
		level    AssuranceLevel
		aal      AAL
		phishing bool
	}{
		{L1, AAL1, false},
		{L2, AAL2, false},
		{L3, AAL3, true},
	}
	for _, c := range cases {
		if got := c.level.RequiredAAL(); got != c.aal {
			t.Errorf("%s.RequiredAAL() = %s, quero %s", c.level, got, c.aal)
		}
		if got := c.level.RequiresPhishingResistant(); got != c.phishing {
			t.Errorf("%s.RequiresPhishingResistant() = %v, quero %v", c.level, got, c.phishing)
		}
	}
}

// Nível não reconhecido (inclui o zero-value) é fail-closed: exige AAL3 e
// phishing-resistant, nunca o mais fraco.
func TestUnknownLevelIsFailClosed(t *testing.T) {
	var zero AssuranceLevel
	if zero.Valid() {
		t.Fatalf("nível vazio não deveria ser válido")
	}
	if zero.RequiredAAL() != AAL3 || !zero.RequiresPhishingResistant() {
		t.Fatalf("nível desconhecido deveria exigir o mais forte (AAL3 + phishing-resistant)")
	}
}

// Satisfies checa AAL e resistência a phishing.
func TestAssuranceLevelSatisfies(t *testing.T) {
	// L3 exige AAL3 E phishing-resistant.
	if L3.Satisfies(AAL3, false) {
		t.Fatalf("L3 não deveria ser satisfeito sem fator phishing-resistant")
	}
	if !L3.Satisfies(AAL3, true) {
		t.Fatalf("L3 deveria ser satisfeito por AAL3 phishing-resistant")
	}
	// TOTP em L3: AAL2 não alcança AAL3 — recusado (cenário "TOTP em operação L3").
	if L3.Satisfies(AAL2, false) {
		t.Fatalf("AAL2 (TOTP) não deveria satisfazer L3")
	}
	// L2 aceita AAL2 sem exigir phishing resistance.
	if !L2.Satisfies(AAL2, false) {
		t.Fatalf("L2 deveria ser satisfeito por AAL2")
	}
	// L1 aceita qualquer sessão válida.
	if !L1.Satisfies(AAL1, false) {
		t.Fatalf("L1 deveria ser satisfeito por AAL1")
	}
}

func TestOperationCatalogRegisterAndLookup(t *testing.T) {
	cat := NewOperationCatalog()
	if err := cat.Register(Operation{ID: "audit.export", Level: L3, Description: "exporta trilha"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := cat.Register(Operation{ID: "profile.read", Level: L1}); err != nil {
		t.Fatalf("Register L1: %v", err)
	}

	lvl, err := cat.Level("audit.export")
	if err != nil || lvl != L3 {
		t.Fatalf("Level(audit.export) = %s, %v; quero L3", lvl, err)
	}

	// Operação não classificada: DENIAL, não miss silencioso.
	if _, err := cat.Level("session.open"); !errors.Is(err, ErrOperationNotClassified) {
		t.Fatalf("op não classificada: err = %v, quero ErrOperationNotClassified", err)
	}

	// Ids ordenados — o conjunto que o invariante de completude (T-017) compara.
	ids := cat.IDs()
	if len(ids) != 2 || ids[0] != "audit.export" || ids[1] != "profile.read" {
		t.Fatalf("IDs() = %v, quero [audit.export profile.read]", ids)
	}
}

func TestOperationCatalogRejectsMalformedAndDuplicate(t *testing.T) {
	cat := NewOperationCatalog()
	if err := cat.Register(Operation{ID: "", Level: L1}); !errors.Is(err, ErrOperationInvalid) {
		t.Fatalf("id vazio: err = %v, quero ErrOperationInvalid", err)
	}
	if err := cat.Register(Operation{ID: "x", Level: AssuranceLevel("L9")}); !errors.Is(err, ErrOperationInvalid) {
		t.Fatalf("nível inválido: err = %v, quero ErrOperationInvalid", err)
	}
	// Zero-value de nível também é inválido — nada de default implícito.
	if err := cat.Register(Operation{ID: "y"}); !errors.Is(err, ErrOperationInvalid) {
		t.Fatalf("nível vazio: err = %v, quero ErrOperationInvalid", err)
	}
	if err := cat.Register(Operation{ID: "z", Level: L2}); err != nil {
		t.Fatalf("Register z: %v", err)
	}
	if err := cat.Register(Operation{ID: "z", Level: L3}); !errors.Is(err, ErrOperationDuplicate) {
		t.Fatalf("duplicata: err = %v, quero ErrOperationDuplicate", err)
	}
}

// activeSessionWithContext: sessão ativa com contexto de autenticação registrado.
func activeSessionWithContext(t *testing.T, aal AAL, methods ...FactorType) AuthSession {
	t.Helper()
	s := activeSession(t, aal)
	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, methods); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	return s
}

func newGuard(t *testing.T) *AssuranceGuard {
	t.Helper()
	cat := NewOperationCatalog()
	for _, op := range []Operation{
		{ID: "profile.read", Level: L1},
		{ID: "tenant.admin", Level: L2},
		{ID: "audit.export", Level: L3},
	} {
		if err := cat.Register(op); err != nil {
			t.Fatalf("Register %s: %v", op.ID, err)
		}
	}
	return NewAssuranceGuard(cat)
}

// Uma sessão WebAuthn AAL3 satisfaz uma operação L3.
func TestGuardAllowsSufficient(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	if err := g.Authorize("audit.export", &s); err != nil {
		t.Fatalf("sessão AAL3 WebAuthn deveria satisfazer L3: %v", err)
	}
}

// TOTP AAL2 numa operação L3: recusa ESPECÍFICA que informa o acr exigido e que
// precisa de fator resistente a phishing (cenário "TOTP em operação L3").
func TestGuardDeniesWithSpecificError(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL2, FactorTOTP)
	err := g.Authorize("audit.export", &s)
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("erro = %v, quero InsufficientAssuranceError", err)
	}
	if iae.Required != L3 || iae.RequiredACR != "aal3" || !iae.NeedsPhishingResistant {
		t.Fatalf("erro deveria exigir L3/aal3/phishing-resistant: %+v", iae)
	}
	if iae.ProvenACR != "aal2" {
		t.Fatalf("erro deveria informar o acr atual aal2: %+v", iae)
	}
}

// Operação não classificada: recusada com ErrOperationNotClassified (nunca
// liberada).
func TestGuardDeniesUnclassified(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	if err := g.Authorize("session.open", &s); !errors.Is(err, ErrOperationNotClassified) {
		t.Fatalf("op não classificada: err = %v, quero ErrOperationNotClassified", err)
	}
}

// Sessão nil ou não-ativa não carrega garantia — recusada.
func TestGuardDeniesNilOrInactiveSession(t *testing.T) {
	g := newGuard(t)
	if err := g.Authorize("profile.read", nil); err == nil {
		t.Fatalf("sessão nil deveria ser recusada")
	}
	revoked := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	revoked.Revoke()
	var iae *InsufficientAssuranceError
	if err := g.Authorize("profile.read", &revoked); !errors.As(err, &iae) {
		t.Fatalf("sessão revogada: err = %v, quero InsufficientAssuranceError", err)
	}
}
