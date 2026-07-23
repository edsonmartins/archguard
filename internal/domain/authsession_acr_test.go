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
	"testing"
	"time"

	"github.com/google/uuid"
)

func activeSession(t *testing.T, aal AAL) AuthSession {
	t.Helper()
	id := uuid.New()
	org := uuid.New()
	m, err := NewMembership(id, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	s, err := NewAuthSession(id, aal, []Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	return s
}

// WebAuthn de hardware (AAL3): acr = aal3, amr = [hwk]. Um único fator forte
// não é "mfa".
func TestACRAMRWebAuthnHardware(t *testing.T) {
	s := activeSession(t, AAL3)
	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	if s.ACR() != "L3" {
		t.Fatalf("acr = %q, quero L3", s.ACR())
	}
	if got := s.AMR(); len(got) != 1 || got[0] != "hwk" {
		t.Fatalf("amr = %v, quero [hwk]", got)
	}
	if !s.AuthTime.Equal(at) {
		t.Fatalf("auth_time = %v, quero %v", s.AuthTime, at)
	}
}

// Senha + TOTP (AAL2): amr = [pwd otp mfa], na ordem provada.
func TestACRAMRPasswordPlusTOTPIsMFA(t *testing.T) {
	s := activeSession(t, AAL2)
	at := time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorPassword, FactorTOTP}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	if s.ACR() != "L2" {
		t.Fatalf("acr = %q, quero L2", s.ACR())
	}
	got := s.AMR()
	want := []string{"pwd", "otp", "mfa"}
	if len(got) != len(want) {
		t.Fatalf("amr = %v, quero %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("amr[%d] = %q, quero %q (amr=%v)", i, got[i], want[i], got)
		}
	}
}

// Recovery code não tem token amr padrão — mas continua sendo AAL2 no acr.
func TestACRAMRRecoveryCodeHasNoToken(t *testing.T) {
	s := activeSession(t, AAL2)
	at := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorRecoveryCode}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	if s.ACR() != "L2" {
		t.Fatalf("acr = %q, quero L2", s.ACR())
	}
	if got := s.AMR(); len(got) != 0 {
		t.Fatalf("amr = %v, quero vazio", got)
	}
}

// Honestidade do acr: uma sessão AAL2 não pode ser sustentada só por senha
// (teto aal1) — SetAuthContext recusa.
func TestSetAuthContextRejectsAALAboveMethodCeiling(t *testing.T) {
	s := activeSession(t, AAL2)
	at := time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorPassword}); err == nil {
		t.Fatalf("AAL2 sustentado só por senha deveria ser recusado")
	}
	// Métodos vazios e auth_time zero também são recusados.
	if err := s.SetAuthContext(at, nil); err == nil {
		t.Fatalf("nenhum método deveria ser recusado")
	}
	if err := s.SetAuthContext(time.Time{}, []FactorType{FactorWebAuthn}); err == nil {
		t.Fatalf("auth_time zero deveria ser recusado")
	}
}

// Sem contexto registrado (ou AAL inválido) não se afirma acr — fail-closed.
func TestACRUnsetIsEmpty(t *testing.T) {
	s := AuthSession{}
	if s.ACR() != "" {
		t.Fatalf("sessão sem AAL não deveria afirmar acr")
	}
	if got := s.AMR(); len(got) != 0 {
		t.Fatalf("sessão sem métodos não deveria ter amr, veio %v", got)
	}
}
