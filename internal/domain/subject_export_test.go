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

	"github.com/google/uuid"
)

// O documento de export carrega a identidade global + UMA organização — cross-tenant
// é impossível por construção.
func TestBuildSubjectExport(t *testing.T) {
	org := uuid.New()
	doc := BuildSubjectExport(
		ExportedIdentity{Subject: "subj", Email: "ana@cli.com", DisplayName: "Ana", Type: "human", Status: "active"},
		ExportedMembership{OrganizationID: org.String(), Status: "active"},
	)
	if doc.Identity.Email != "ana@cli.com" || doc.Organization.OrganizationID != org.String() {
		t.Fatalf("documento inesperado: %+v", doc)
	}
	// A estrutura só admite UMA organização — não há campo para dados de outro tenant.
}

// A requisição de export é auditada (subject.export, L2) só com pseudônimos.
func TestSubjectExportAudit(t *testing.T) {
	org := uuid.New()
	in := BuildSubjectExportAuditInput(org, "operador-opaco", "titular-opaco")
	if in.Action != ActionSubjectExport || in.Context.AuthContextClass != "L2" {
		t.Fatalf("evento de export inesperado: %+v", in)
	}
	if in.Target.ID != "titular-opaco" || in.Actor.IdentitySubject != "operador-opaco" {
		t.Fatalf("auditoria de export deveria carregar só pseudônimos: %+v", in)
	}
	if ActionSubjectExport.AssuranceLevel() != L2 {
		t.Fatalf("subject.export deveria ser L2")
	}
	ev, err := NewAuditEvent(in)
	if err != nil || ev.SerializedOutcome() != "success" {
		t.Fatalf("evento de export deveria ser válido: %v", err)
	}
}
