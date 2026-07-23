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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestDelegation(t *testing.T, nb, exp time.Time) Delegation {
	t.Helper()
	d, err := NewDelegation(uuid.New(), uuid.New(), "sub-admin", uuid.New(), "sub-target", nb, exp)
	if err != nil {
		t.Fatalf("NewDelegation: %v", err)
	}
	return d
}

func TestNewDelegationStartsPendingConsent(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(15*time.Minute))
	if d.Status != DelegationPendingConsent {
		t.Fatalf("delegação deveria nascer pending_consent, veio %s", d.Status)
	}
}

func TestNewDelegationValidation(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(time.Hour)
	// Ator real == alvo é recusado.
	if _, err := NewDelegation(uuid.New(), uuid.New(), "same", uuid.New(), "same", nb, exp); !errors.Is(err, ErrInvalidDelegation) {
		t.Fatalf("mesmo sujeito: err = %v, quero ErrInvalidDelegation", err)
	}
	// Janela invertida.
	if _, err := NewDelegation(uuid.New(), uuid.New(), "a", uuid.New(), "b", exp, nb); !errors.Is(err, ErrInvalidDelegation) {
		t.Fatalf("janela invertida: err = %v", err)
	}
}

// O token de delegação carrega o sujeito impersonado em sub e o ator real em act
// (cenário "Ação sob delegação").
func TestDelegationTokenClaimsCarryActAndSub(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(15 * time.Minute)
	d := newTestDelegation(t, nb, exp)

	// Pendente de consentimento: NÃO emite token (fail-closed).
	if _, err := d.TokenClaims(nb.Add(time.Minute)); !errors.Is(err, ErrDelegationNotActive) {
		t.Fatalf("pendente: err = %v, quero ErrDelegationNotActive", err)
	}

	d.Status = DelegationActive
	claims, err := d.TokenClaims(nb.Add(time.Minute))
	if err != nil {
		t.Fatalf("TokenClaims: %v", err)
	}
	if claims.Sub != "sub-target" {
		t.Fatalf("sub deveria ser o sujeito impersonado, veio %q", claims.Sub)
	}
	if claims.Act.Sub != "sub-admin" {
		t.Fatalf("act.sub deveria ser o ator real, veio %q", claims.Act.Sub)
	}
	if !claims.Delegated {
		t.Fatalf("o token deveria estar marcado como delegação")
	}

	// Fora da janela (expirada): não emite token mesmo com status active.
	if _, err := d.TokenClaims(exp); !errors.Is(err, ErrDelegationNotActive) {
		t.Fatalf("expirada: err = %v, quero ErrDelegationNotActive", err)
	}
}

// A auditoria de uma ação delegada registra ambos: sujeito impersonado e ator
// real (no Act) — o não-repúdio reconstrói quem executou.
func TestDelegationAuditActorRecordsBoth(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(time.Hour))
	actor := d.AuditActor()
	if actor.IdentitySubject != "sub-target" {
		t.Fatalf("ator aparente deveria ser o impersonado, veio %q", actor.IdentitySubject)
	}
	if actor.Act == nil || actor.Act.IdentitySubject != "sub-admin" {
		t.Fatalf("o ator real deveria estar em Act, veio %+v", actor.Act)
	}
}

// Consentimento é o gate: a delegação só ativa (e só emite token) após o alvo
// consentir; sem consentimento nenhum token é emitido (cenário "Delegação
// padrão").
func TestDelegationConsentGate(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(15*time.Minute))

	// Sem consentimento não emite token.
	if _, err := d.TokenClaims(nb.Add(time.Minute)); !errors.Is(err, ErrDelegationNotActive) {
		t.Fatalf("sem consentimento não deveria emitir token: %v", err)
	}

	if err := d.Consent(); err != nil {
		t.Fatalf("Consent: %v", err)
	}
	if d.Status != DelegationActive {
		t.Fatalf("após consentir deveria estar ativa, veio %s", d.Status)
	}
	if _, err := d.TokenClaims(nb.Add(time.Minute)); err != nil {
		t.Fatalf("após consentir deveria emitir token: %v", err)
	}

	// Consentir de novo (já ativa) é transição inválida.
	if err := d.Consent(); !errors.Is(err, ErrDelegationTransition) {
		t.Fatalf("consentir ativa: err = %v, quero ErrDelegationTransition", err)
	}
}

// Recusa do alvo: a delegação vai para denied e nunca inicia.
func TestDelegationDenyConsent(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(15*time.Minute))
	if err := d.DenyConsent(); err != nil {
		t.Fatalf("DenyConsent: %v", err)
	}
	if d.Status != DelegationDenied {
		t.Fatalf("status = %s, quero denied", d.Status)
	}
	if _, err := d.TokenClaims(nb.Add(time.Minute)); !errors.Is(err, ErrDelegationNotActive) {
		t.Fatalf("delegação recusada não deveria emitir token")
	}
}

// A notificação de início vai ao alvo, nomeando o ator real, sem dado pessoal;
// e o banner permanente nomeia ambos (cenário "Delegação padrão", T-005).
func TestDelegationNotificationAndBanner(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(15*time.Minute))

	n := d.StartedNotification()
	if n.Recipient != "sub-target" || n.Kind != NotifyDelegationStarted {
		t.Fatalf("notificação de início inesperada: %+v", n)
	}
	if !strings.Contains(n.Detail, "sub-admin") {
		t.Fatalf("a notificação deveria nomear o ator real: %q", n.Detail)
	}
	banner := d.SessionBanner()
	if !strings.Contains(banner, "sub-target") || !strings.Contains(banner, "sub-admin") {
		t.Fatalf("o banner deveria nomear ambos: %q", banner)
	}
}

// Revogação encerra a sessão delegada imediatamente: um token deixa de ser
// emitido (cenário "Revogação pelo alvo").
func TestDelegationRevokeEndsSessionImmediately(t *testing.T) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	d := newTestDelegation(t, nb, nb.Add(time.Hour))
	if err := d.Consent(); err != nil {
		t.Fatalf("Consent: %v", err)
	}
	// Ativa e vigente: emite token.
	if _, err := d.TokenClaims(nb.Add(time.Minute)); err != nil {
		t.Fatalf("pré-condição: deveria emitir token: %v", err)
	}

	d.Revoke()
	if d.Status != DelegationRevoked {
		t.Fatalf("status = %s, quero revoked", d.Status)
	}
	// Imediatamente após revogar, nenhum token é emitido (sessão encerrada).
	if _, err := d.TokenClaims(nb.Add(2 * time.Minute)); !errors.Is(err, ErrDelegationNotActive) {
		t.Fatalf("delegação revogada não deveria emitir token: %v", err)
	}

	// Idempotente e não reativa uma delegação revogada.
	d.Revoke()
	if d.Status != DelegationRevoked {
		t.Fatalf("revoke deveria ser idempotente")
	}
}
