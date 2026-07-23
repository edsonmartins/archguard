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

func TestNewRecoveryRequestValidation(t *testing.T) {
	target, org, req := uuid.New(), uuid.New(), uuid.New()

	// Justificativa obrigatória.
	if _, err := NewRecoveryRequest(target, org, req, "", 2); !errors.Is(err, ErrInvalidRecoveryRequest) {
		t.Fatalf("sem justificativa: err = %v", err)
	}
	// Default de aprovações quando 0.
	r, err := NewRecoveryRequest(target, org, req, "perdi a chave", 0)
	if err != nil {
		t.Fatalf("NewRecoveryRequest: %v", err)
	}
	if r.RequiredApprovals != DefaultRecoveryApprovals || r.Status != RecoveryPending {
		t.Fatalf("request inesperado: %+v", r)
	}
}

// Fluxo feliz: duas aprovações de pares distintos → aprovado → consumido.
func TestRecoveryApprovalThreshold(t *testing.T) {
	target, org, req := uuid.New(), uuid.New(), uuid.New()
	r, _ := NewRecoveryRequest(target, org, req, "perdi o autenticador", 2)

	peer1, peer2 := uuid.New(), uuid.New()
	if err := r.Approve(peer1); err != nil {
		t.Fatalf("primeira aprovação: %v", err)
	}
	if r.Status != RecoveryPending {
		t.Fatalf("uma aprovação não deveria bastar: %s", r.Status)
	}
	if err := r.Approve(peer2); err != nil {
		t.Fatalf("segunda aprovação: %v", err)
	}
	if r.Status != RecoveryApproved {
		t.Fatalf("duas aprovações deveriam aprovar: %s", r.Status)
	}

	// Reset só após aprovado.
	if err := r.MarkConsumed(); err != nil {
		t.Fatalf("consumo após aprovação: %v", err)
	}
	if r.Status != RecoveryConsumed {
		t.Fatalf("status = %s, quero consumed", r.Status)
	}
}

// Separação de deveres: alvo e solicitante não podem aprovar; sem duplicata.
func TestRecoverySeparationOfDuties(t *testing.T) {
	target, org, req := uuid.New(), uuid.New(), uuid.New()
	r, _ := NewRecoveryRequest(target, org, req, "perdi", 2)

	if err := r.Approve(target); !errors.Is(err, ErrApproverIsTarget) {
		t.Fatalf("alvo aprovando: err = %v", err)
	}
	if err := r.Approve(req); !errors.Is(err, ErrApproverIsRequester) {
		t.Fatalf("solicitante aprovando: err = %v", err)
	}
	peer := uuid.New()
	if err := r.Approve(peer); err != nil {
		t.Fatalf("par aprovando: %v", err)
	}
	if err := r.Approve(peer); !errors.Is(err, ErrDuplicateApproval) {
		t.Fatalf("aprovação duplicada: err = %v", err)
	}
}

// Reset sem aprovação é recusado — nenhum caminho reseta um fator sem passar
// pela aprovação de pares (cenário "reset silencioso").
func TestRecoveryConsumeRequiresApproval(t *testing.T) {
	target, org, req := uuid.New(), uuid.New(), uuid.New()
	r, _ := NewRecoveryRequest(target, org, req, "perdi", 2)

	if err := r.MarkConsumed(); !errors.Is(err, ErrRecoveryNotApproved) {
		t.Fatalf("consumo sem aprovação: err = %v, quero ErrRecoveryNotApproved", err)
	}
	// Uma aprovação (abaixo do limiar) ainda não permite o reset.
	_ = r.Approve(uuid.New())
	if err := r.MarkConsumed(); !errors.Is(err, ErrRecoveryNotApproved) {
		t.Fatalf("consumo abaixo do limiar: err = %v", err)
	}
}

// Rejeitada é terminal para novas aprovações.
func TestRecoveryRejectIsTerminal(t *testing.T) {
	target, org, req := uuid.New(), uuid.New(), uuid.New()
	r, _ := NewRecoveryRequest(target, org, req, "suspeita de fraude", 2)
	if err := r.Reject(); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if err := r.Approve(uuid.New()); !errors.Is(err, ErrRecoveryNotPending) {
		t.Fatalf("aprovar rejeitada: err = %v, quero ErrRecoveryNotPending", err)
	}
	if err := r.MarkConsumed(); !errors.Is(err, ErrRecoveryNotApproved) {
		t.Fatalf("consumir rejeitada: err = %v", err)
	}
}
