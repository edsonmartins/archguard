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
	"strings"
	"testing"

	"github.com/google/uuid"
)

func decisionAudit() DecisionAudit {
	return DecisionAudit{
		OrganizationID:          uuid.New(),
		Action:                  ActionPrivilegedSessionOpen,
		Actor:                   AuditActor{IdentitySubject: "subj-opaco"},
		Target:                  AuditTarget{Type: "asset", ID: "db-prod-03", Label: "host privilegiado"},
		ACR:                     "L2",
		PrivilegedCorrelationID: "pcid-123",
	}
}

// Decisão permitida: outcome success + justificativa anexada; evento válido.
func TestDecisionAuditAllowed(t *testing.T) {
	in := BuildDecisionAuditInput(decisionAudit(), Allow("operator from parent asset_group:g1"), nil)
	if in.Outcome != Allowed {
		t.Fatalf("permitido deveria ser Allowed")
	}
	if !strings.Contains(in.Reason, "operator from parent") {
		t.Fatalf("a justificativa deveria acompanhar o evento: %q", in.Reason)
	}
	ev, err := NewAuditEvent(in)
	if err != nil {
		t.Fatalf("o evento de decisão deveria ser válido: %v", err)
	}
	if ev.SerializedOutcome() != "success" || ev.Context.PrivilegedCorrelationID != "pcid-123" {
		t.Fatalf("serialização/contexto inesperados: %s %+v", ev.SerializedOutcome(), ev.Context)
	}
}

// Recusa computada: outcome denied.
func TestDecisionAuditDenied(t *testing.T) {
	in := BuildDecisionAuditInput(decisionAudit(), DenyDecision("sem relação"), nil)
	if in.Outcome != Denied {
		t.Fatalf("recusa computada deveria ser Denied")
	}
	ev, _ := NewAuditEvent(in)
	if ev.SerializedOutcome() != "denied" {
		t.Fatalf("serialização deveria ser denied, veio %s", ev.SerializedOutcome())
	}
}

// Falha de infraestrutura: outcome error, distinto de denied (INV-6), razão
// genérica (sem vazar detalhe de infraestrutura, INV-7).
func TestDecisionAuditError(t *testing.T) {
	in := BuildDecisionAuditInput(decisionAudit(), Decision{}, ErrPDPUnavailable)
	if in.Outcome != Failed {
		t.Fatalf("falha do PDP deveria ser Failed (error), não Denied")
	}
	ev, _ := NewAuditEvent(in)
	if ev.SerializedOutcome() != "error" {
		t.Fatalf("serialização deveria ser error, veio %s", ev.SerializedOutcome())
	}
	if strings.Contains(in.Reason, "indisponível") == false {
		t.Fatalf("a razão deveria indicar indisponibilidade fail-closed: %q", in.Reason)
	}
}
