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

// Fluxo completo "Step-up concluído": uma sessão TOTP AAL2 é recusada numa
// operação L3; após step-up WebAuthn, a MESMA operação passa e o acr reflete o
// nível obtido — a operação original é retomada sem perda de contexto (mesma
// op, mesmo tenant; só a garantia da sessão subiu).
func TestStepUpResumesOriginalOperation(t *testing.T) {
	cat := NewOperationCatalog()
	if err := cat.Register(Operation{ID: "audit.export", Level: L3}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	g := NewAssuranceGuard(cat)

	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	s := activeSession(t, AAL2)
	if err := s.SetAuthContext(at, []FactorType{FactorTOTP}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}

	// Recusa: TOTP AAL2 não satisfaz L3.
	var iae *InsufficientAssuranceError
	if err := g.Authorize("audit.export", &s, at.Add(time.Minute)); !errors.As(err, &iae) {
		t.Fatalf("pré-step-up: err = %v, quero InsufficientAssuranceError", err)
	}

	// Step-up WebAuthn no instante da reautenticação.
	stepAt := at.Add(2 * time.Minute)
	if err := s.StepUp(stepAt, AAL3, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("StepUp: %v", err)
	}

	// A operação original agora passa, e o acr reflete o nível obtido.
	if err := g.Authorize("audit.export", &s, stepAt.Add(time.Minute)); err != nil {
		t.Fatalf("pós-step-up a operação deveria passar: %v", err)
	}
	if s.ACR() != "aal3" {
		t.Fatalf("acr = %q, quero aal3 após step-up", s.ACR())
	}
	if got := s.AMR(); len(got) != 1 || got[0] != "hwk" {
		t.Fatalf("amr = %v, quero [hwk] após step-up WebAuthn", got)
	}
}

// Step-up de refresh: uma sessão L3 obsoleta reautentica com WebAuthn, mantém
// AAL3 e recupera o frescor.
func TestStepUpRefreshesFreshness(t *testing.T) {
	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	s := activeSession(t, AAL3)
	if err := s.SetAuthContext(at, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	refreshAt := at.Add(30 * time.Minute)
	if err := s.StepUp(refreshAt, AAL3, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("StepUp refresh: %v", err)
	}
	if !s.AuthTime.Equal(refreshAt) {
		t.Fatalf("auth_time = %v, quero %v após refresh", s.AuthTime, refreshAt)
	}
	if !L3.Fresh(s.AuthTime, refreshAt.Add(time.Minute)) {
		t.Fatalf("L3 deveria estar fresco logo após o refresh")
	}
}

// Um step-up não pode REDUZIR a garantia, nem inflar acr além dos fatores.
func TestStepUpGuards(t *testing.T) {
	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	s := activeSession(t, AAL3)
	if err := s.SetAuthContext(at, []FactorType{FactorWebAuthn}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	// Reduzir de AAL3 para AAL2 é recusado.
	if err := s.StepUp(at.Add(time.Minute), AAL2, []FactorType{FactorTOTP}); !errors.Is(err, ErrStepUpLowersAssurance) {
		t.Fatalf("redução: err = %v, quero ErrStepUpLowersAssurance", err)
	}
	// A garantia original permaneceu intacta após a recusa.
	if s.ProvenAAL != AAL3 {
		t.Fatalf("garantia deveria seguir AAL3 após step-up recusado, veio %s", s.ProvenAAL)
	}

	// Inflar AAL3 só com TOTP (teto aal2) é recusado — e faz rollback do nível.
	s2 := activeSession(t, AAL2)
	if err := s2.SetAuthContext(at, []FactorType{FactorTOTP}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	if err := s2.StepUp(at.Add(time.Minute), AAL3, []FactorType{FactorTOTP}); err == nil {
		t.Fatalf("step-up AAL3 só com TOTP deveria ser recusado")
	}
	if s2.ProvenAAL != AAL2 {
		t.Fatalf("nível deveria ter feito rollback para AAL2, veio %s", s2.ProvenAAL)
	}

	// Step-up numa sessão revogada é recusado.
	s3 := activeSession(t, AAL2)
	s3.Revoke()
	if err := s3.StepUp(at, AAL3, []FactorType{FactorWebAuthn}); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("sessão revogada: err = %v, quero ErrSessionRevoked", err)
	}
}
