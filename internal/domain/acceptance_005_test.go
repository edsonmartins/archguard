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

// Testes de ACEITAÇÃO dos cenários nomeados do pacote 005 (T-018/019/020),
// escritos contra o catálogo canônico de operações (BuildOperationCatalog), a
// mesma classificação que a API usa em produção — não uma montagem de teste.

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// acceptanceSession: uma sessão ativa comprovando `aal` com os `methods` dados,
// autenticada em `at`.
func acceptanceSession(t *testing.T, aal AAL, at time.Time, methods ...FactorType) AuthSession {
	t.Helper()
	id, org := uuid.New(), uuid.New()
	m, err := NewMembership(id, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	s, err := NewAuthSession(id, aal, []Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := s.SetAuthContext(at, methods); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	return s
}

// T-018 — "Operação L3 com sessão antiga": um usuário com sessão válida e ANTIGA
// (mas com o fator certo, WebAuthn AAL3) que solicita abrir sessão privilegiada
// (L3) é RECUSADO por garantia insuficiente e informado do acr exigido; após
// reautenticar (step-up), a operação passa.
func TestAcceptanceL3StaleSessionRequiresReauth(t *testing.T) {
	cat, err := BuildOperationCatalog()
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	g := NewAssuranceGuard(cat)

	login := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	s := acceptanceSession(t, AAL3, login, FactorWebAuthn)

	// Muito depois do login (além da janela curta de L3): recusa por frescor.
	old := login.Add(time.Hour)
	err = g.Authorize(string(ActionPrivilegedSessionOpen), &s, AAL1, old)
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("sessão antiga deveria ser recusada: %v", err)
	}
	if !iae.Stale || iae.RequiredACR != "L3" {
		t.Fatalf("recusa deveria ser por frescor, exigindo acr aal3: %+v", iae)
	}

	// Reautentica (step-up WebAuthn) e retoma a operação.
	if err := s.StepUp(old, AAL3, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("StepUp: %v", err)
	}
	if err := g.Authorize(string(ActionPrivilegedSessionOpen), &s, AAL1, old.Add(time.Minute)); err != nil {
		t.Fatalf("após reautenticação a operação deveria passar: %v", err)
	}
}

// T-019 — "TOTP recusado em operação L3": uma sessão que só comprovou TOTP (AAL2)
// não satisfaz uma operação L3 (exportar auditoria); a recusa exige explicitamente
// fator resistente a phishing (WebAuthn).
func TestAcceptanceTOTPDeniedAtL3(t *testing.T) {
	cat, err := BuildOperationCatalog()
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	g := NewAssuranceGuard(cat)

	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	s := acceptanceSession(t, AAL2, now, FactorTOTP)

	err = g.Authorize(string(ActionAuditExport), &s, AAL1, now.Add(time.Minute))
	var iae *InsufficientAssuranceError
	if !errors.As(err, &iae) {
		t.Fatalf("TOTP em L3 deveria ser recusado: %v", err)
	}
	if !iae.NeedsPhishingResistant || iae.RequiredACR != "L3" {
		t.Fatalf("a recusa deveria exigir WebAuthn (aal3, phishing-resistant): %+v", iae)
	}
}

// T-020 — "nenhum caminho de reset administrativo silencioso de fator": os DOIS
// caminhos que resetam/removem um fator forte são estruturalmente não-silenciosos.
// (a) a remoção administrativa é uma operação L3 (privilegiada — auditada e com
// step-up; a atomicidade da auditoria é provada na integração do FactorRemover).
// (b) a recuperação NÃO reseta sem aprovação de pares: MarkConsumed exige aprovado.
func TestAcceptanceNoSilentAdminFactorReset(t *testing.T) {
	cat, err := BuildOperationCatalog()
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}

	// (a) remover fator é L3 — nunca uma operação trivial e silenciosa.
	lvl, err := cat.Level(string(ActionFactorRemove))
	if err != nil || lvl != L3 {
		t.Fatalf("factor.remove deveria ser operação L3: %s, %v", lvl, err)
	}
	// factor.remove NÃO é permitida durante enrolamento (não é registro de fator).
	if op, _ := cat.Lookup(string(ActionFactorRemove)); op.AllowedDuringEnrollment {
		t.Fatalf("remoção de fator não deveria ser permitida no estado de enrolamento")
	}

	// (b) recuperação: sem aprovação de pares não há reset.
	target, org, requester := uuid.New(), uuid.New(), uuid.New()
	r, err := NewRecoveryRequest(target, org, requester, "perdi o autenticador", 2)
	if err != nil {
		t.Fatalf("NewRecoveryRequest: %v", err)
	}
	if err := r.MarkConsumed(); !errors.Is(err, ErrRecoveryNotApproved) {
		t.Fatalf("reset sem aprovação deveria ser recusado: %v", err)
	}
	// Uma aprovação (abaixo do limiar) ainda não permite o reset.
	if err := r.Approve(uuid.New()); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := r.MarkConsumed(); !errors.Is(err, ErrRecoveryNotApproved) {
		t.Fatalf("reset abaixo do limiar deveria ser recusado: %v", err)
	}
}
