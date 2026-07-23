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

// testAuthTime is when the sessions in these tests authenticated; testNow is a
// few minutes later — within every level's freshness window, so freshness does
// not interfere with the non-freshness assertions.
var (
	testAuthTime = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	testNow      = testAuthTime.Add(2 * time.Minute)
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
	// Frescor também é fail-closed no nível desconhecido: janela curta (a de L3).
	if !zero.RequiresFreshness() || zero.FreshnessWindow() != freshnessL3Window {
		t.Fatalf("nível desconhecido deveria exigir frescor curto")
	}
}

// Satisfies checa AAL e resistência a phishing.
func TestAssuranceLevelSatisfies(t *testing.T) {
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
	if !L2.Satisfies(AAL2, false) {
		t.Fatalf("L2 deveria ser satisfeito por AAL2")
	}
	if !L1.Satisfies(AAL1, false) {
		t.Fatalf("L1 deveria ser satisfeito por AAL1")
	}
}

// Frescor por nível: L1 sempre fresco; L2 janela generosa; L3 janela curta;
// auth_time zero ou no futuro nunca é fresco (fail-closed).
func TestAssuranceLevelFresh(t *testing.T) {
	base := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	// L1 dispensa frescor mesmo com autenticação antiga.
	if !L1.Fresh(base.Add(-100*time.Hour), base) {
		t.Fatalf("L1 não deveria exigir frescor")
	}
	// L3 fresco dentro de 5min, obsoleto além.
	if !L3.Fresh(base.Add(-4*time.Minute), base) {
		t.Fatalf("L3 dentro de 5min deveria ser fresco")
	}
	if L3.Fresh(base.Add(-6*time.Minute), base) {
		t.Fatalf("L3 além de 5min deveria estar obsoleto")
	}
	// L2 fresco dentro de 12h, obsoleto além.
	if !L2.Fresh(base.Add(-11*time.Hour), base) {
		t.Fatalf("L2 dentro de 12h deveria ser fresco")
	}
	if L2.Fresh(base.Add(-13*time.Hour), base) {
		t.Fatalf("L2 além de 12h deveria estar obsoleto")
	}
	// Fail-closed: auth_time zero e auth_time no futuro nunca são frescos.
	if L2.Fresh(time.Time{}, base) {
		t.Fatalf("auth_time zero não deveria ser fresco")
	}
	if L3.Fresh(base.Add(time.Minute), base) {
		t.Fatalf("auth_time no futuro não deveria ser fresco")
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

	if _, err := cat.Level("session.open"); !errors.Is(err, ErrOperationNotClassified) {
		t.Fatalf("op não classificada: err = %v, quero ErrOperationNotClassified", err)
	}

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

// activeSessionWithContext: sessão ativa com contexto de autenticação em
// testAuthTime.
func activeSessionWithContext(t *testing.T, aal AAL, methods ...FactorType) AuthSession {
	t.Helper()
	s := activeSession(t, aal)
	if err := s.SetAuthContext(testAuthTime, methods); err != nil {
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

// Uma sessão WebAuthn AAL3 recente satisfaz uma operação L3.
func TestGuardAllowsSufficient(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	if err := g.Authorize("audit.export", &s, AAL1, testNow); err != nil {
		t.Fatalf("sessão AAL3 WebAuthn recente deveria satisfazer L3: %v", err)
	}
}

// TOTP AAL2 numa operação L3: recusa ESPECÍFICA (não por frescor) que informa o
// acr exigido e a necessidade de fator resistente a phishing.
func TestGuardDeniesWithSpecificError(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL2, FactorTOTP)
	err := g.Authorize("audit.export", &s, AAL1, testNow)
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("erro = %v, quero InsufficientAssuranceError", err)
	}
	if iae.Required != L3 || iae.RequiredACR != "aal3" || !iae.NeedsPhishingResistant {
		t.Fatalf("erro deveria exigir L3/aal3/phishing-resistant: %+v", iae)
	}
	if iae.ProvenACR != "aal2" || iae.Stale {
		t.Fatalf("recusa deveria ser por nível (não frescor), com acr atual aal2: %+v", iae)
	}
}

// Cenário "Operação L3 com sessão antiga": a sessão TEM o fator certo (WebAuthn
// AAL3) mas autenticou há muito — recusa por frescor, marcada Stale, exigindo
// reautenticação.
func TestGuardDeniesStaleSession(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	old := testAuthTime.Add(10 * time.Minute) // 10min > janela L3 de 5min
	err := g.Authorize("audit.export", &s, AAL1, old)
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("erro = %v, quero InsufficientAssuranceError", err)
	}
	if !iae.Stale {
		t.Fatalf("recusa de sessão antiga deveria ser marcada Stale: %+v", iae)
	}
	if iae.RequiredACR != "aal3" {
		t.Fatalf("mesmo obsoleta, o acr exigido para reautenticar é aal3: %+v", iae)
	}
}

// Após reautenticação (auth_time renovado), a mesma operação passa — o step-up
// resolveu o frescor.
func TestGuardStepUpRestoresFreshness(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	stale := testAuthTime.Add(10 * time.Minute)
	if err := g.Authorize("audit.export", &s, AAL1, stale); err == nil {
		t.Fatalf("pré-condição: a operação deveria estar recusada por frescor")
	}
	// Step-up: renova o contexto de autenticação para o instante da reautenticação.
	if err := s.SetAuthContext(stale, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("SetAuthContext (step-up): %v", err)
	}
	if err := g.Authorize("audit.export", &s, AAL1, stale.Add(time.Minute)); err != nil {
		t.Fatalf("após step-up a operação deveria passar: %v", err)
	}
}

// Operação não classificada: recusada com ErrOperationNotClassified.
func TestGuardDeniesUnclassified(t *testing.T) {
	g := newGuard(t)
	s := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	if err := g.Authorize("session.open", &s, AAL1, testNow); !errors.Is(err, ErrOperationNotClassified) {
		t.Fatalf("op não classificada: err = %v, quero ErrOperationNotClassified", err)
	}
}

// Sessão nil ou não-ativa não carrega garantia — recusada.
func TestGuardDeniesNilOrInactiveSession(t *testing.T) {
	g := newGuard(t)
	if err := g.Authorize("profile.read", nil, AAL1, testNow); err == nil {
		t.Fatalf("sessão nil deveria ser recusada")
	}
	revoked := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	revoked.Revoke()
	var iae *InsufficientAssuranceError
	if err := g.Authorize("profile.read", &revoked, AAL1, testNow); !errors.As(err, &iae) {
		t.Fatalf("sessão revogada: err = %v, quero InsufficientAssuranceError", err)
	}
}

// Precedência "mais restritiva vence" (T-011): o piso do tenant eleva o
// requisito de uma operação de nível baixo. Uma operação L1 num tenant que exige
// WebAuthn (piso AAL3) passa a exigir AAL3 — uma sessão TOTP AAL2 é recusada e o
// desafio informa acr aal3.
func TestGuardTenantFloorRaisesRequirement(t *testing.T) {
	g := newGuard(t)
	totp := activeSessionWithContext(t, AAL2, FactorTOTP)

	// Sem piso do tenant, a operação L1 passa para a sessão AAL2.
	if err := g.Authorize("profile.read", &totp, AAL1, testNow); err != nil {
		t.Fatalf("L1 sem piso deveria passar: %v", err)
	}

	// Com piso AAL3, a MESMA operação L1 exige AAL3 — recusa a sessão TOTP.
	err := g.Authorize("profile.read", &totp, AAL3, testNow)
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("piso AAL3 deveria recusar sessão AAL2 em L1: %v", err)
	}
	if iae.RequiredACR != "aal3" || !iae.NeedsPhishingResistant {
		t.Fatalf("o desafio deveria informar aal3 + phishing-resistant: %+v", iae)
	}

	// Uma sessão WebAuthn AAL3 satisfaz o piso.
	web := activeSessionWithContext(t, AAL3, FactorWebAuthn)
	if err := g.Authorize("profile.read", &web, AAL3, testNow); err != nil {
		t.Fatalf("sessão AAL3 deveria satisfazer o piso AAL3: %v", err)
	}
}

// Um tenant lasso NÃO afrouxa uma operação estrita: L3 continua exigindo AAL3
// mesmo com piso AAL1.
func TestGuardStrictOperationNotLoosenedByLaxTenant(t *testing.T) {
	g := newGuard(t)
	totp := activeSessionWithContext(t, AAL2, FactorTOTP)
	if err := g.Authorize("audit.export", &totp, AAL1, testNow); err == nil {
		t.Fatalf("L3 não deveria ser afrouxada por piso AAL1")
	}
}

// Fail-closed: um piso indefinido é tratado como o mais forte (AAL3).
func TestGuardInvalidFloorFailsClosed(t *testing.T) {
	g := newGuard(t)
	totp := activeSessionWithContext(t, AAL2, FactorTOTP)
	if err := g.Authorize("profile.read", &totp, AAL(""), testNow); err == nil {
		t.Fatalf("piso indefinido deveria exigir o mais forte e recusar AAL2")
	}
}
