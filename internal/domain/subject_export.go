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

import "github.com/google/uuid"

// Subject data export (pacote 010, T-021 / ADR-0014 / spec "Atendimento a direitos
// do titular com isolamento"): a titular's structured data for ONE organization.
// The document carries the global identity (shared across tenants) and the
// membership in the requested organization ONLY — never another organization's
// data. Scoping is STRUCTURAL: the document holds a single Organization, so a
// leak of another tenant's data cannot be expressed here.

// ExportedIdentity is the titular's global identity (decrypted from per-subject
// ciphertext at export time; never stored in the clear).
type ExportedIdentity struct {
	Subject     string
	Email       string
	DisplayName string
	Type        string
	Status      string
}

// ExportedMembership is the titular's membership in the ONE requested organization.
type ExportedMembership struct {
	OrganizationID string
	Status         string
}

// SubjectExportDocument is the structured export for one subject in one org.
type SubjectExportDocument struct {
	Identity     ExportedIdentity
	Organization ExportedMembership
}

// BuildSubjectExport assembles the export document. It takes exactly ONE
// organization's membership, so cross-tenant data is impossible by construction.
func BuildSubjectExport(id ExportedIdentity, mem ExportedMembership) SubjectExportDocument {
	return SubjectExportDocument{Identity: id, Organization: mem}
}

// BuildSubjectExportAuditInput builds the audit event for a subject-access request
// (subject.export, L2). It records who requested, for which subject and org, with
// pseudonyms only — the exported personal data itself is never in the audit event.
func BuildSubjectExportAuditInput(organizationID uuid.UUID, operatorSubject, subject string) AuditEventInput {
	return AuditEventInput{
		OrganizationID: organizationID,
		Action:         ActionSubjectExport,
		Actor:          AuditActor{IdentitySubject: operatorSubject},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "subject", ID: subject, Label: "exportação de dados do titular"},
		Reason:         "requisição de acesso do titular, escopo da organização " + organizationID.String(),
		Context:        AuditContext{AuthContextClass: "L2"},
	}
}
