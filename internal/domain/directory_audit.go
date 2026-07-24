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

// Audit-event builders for the inbound flows of pacote 009 (T-016 / spec scenarios
// "o evento é auditado"): a directory sync, a federated login, and a legacy-channel
// access. Each produces a validated AuditEventInput carrying only pseudonymous
// references — org id, opaque actor subject, provider/connector name — never
// personal data or a secret (INV-7). The events are catalogued and INV-8-exempt
// (they are emitted by the flows, not invoked as API operations).

// NewDirectorySyncAuditInput records one directory sync run over an organization.
// detail is a short, non-personal summary (e.g. counts / connector name).
func NewDirectorySyncAuditInput(organizationID uuid.UUID, connectorName, detail string) AuditEventInput {
	return AuditEventInput{
		OrganizationID: organizationID,
		Action:         ActionDirectorySync,
		Actor:          AuditActor{IdentitySubject: "system:directory-sync"},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "directory_connector", ID: connectorName, Label: "sincronismo de diretório"},
		Reason:         detail,
	}
}

// NewFederatedLoginAuditInput records a federated login. The actor is the resolved
// identity subject (opaque); provider names the IdP; the IdP's acr is recorded as
// context but is informational only (never gates L3, T-013).
func NewFederatedLoginAuditInput(organizationID uuid.UUID, actorSubject string, membershipID *uuid.UUID, provider, protocol, idpACR string) AuditEventInput {
	return AuditEventInput{
		OrganizationID: organizationID,
		Action:         ActionFederatedLogin,
		Actor:          AuditActor{IdentitySubject: actorSubject, MembershipID: membershipID},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "identity_provider", ID: provider, Label: "login federado " + protocol},
		Reason:         "login federado via " + provider + " (acr do IdP informativo: " + idpACR + ")",
		Context:        AuditContext{AuthContextClass: "L1"}, // federation establishes identification only
	}
}

// NewLegacyChannelAuditInput records an access through a legacy edge channel,
// FLAGGED as legacy (RFC-0007 §6). The session's AuditFlag identifies the channel;
// the event's assurance context is L1 (a legacy channel never carries L3, T-015).
func NewLegacyChannelAuditInput(organizationID uuid.UUID, actorSubject string, session LegacyChannelSession) AuditEventInput {
	return AuditEventInput{
		OrganizationID: organizationID,
		Action:         ActionLegacyChannelAccess,
		Actor:          AuditActor{IdentitySubject: actorSubject},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "legacy_channel", ID: string(session.Channel), Label: session.AuditFlag()},
		Reason:         "acesso por canal legado — " + session.AuditFlag() + " — não autoriza L3",
		Context:        AuditContext{AuthContextClass: "L1"},
	}
}
