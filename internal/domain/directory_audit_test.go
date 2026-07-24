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

// Os três eventos inbound do pacote 009 constroem eventos de auditoria válidos e
// catalogados (spec "o evento é auditado").
func TestDirectoryAuditBuilders(t *testing.T) {
	org := uuid.New()
	mem := uuid.New()

	sync := NewDirectorySyncAuditInput(org, "AD Corp", "3 criados, 1 suspenso")
	fed := NewFederatedLoginAuditInput(org, "subj-opaco", &mem, "entra", "saml", "urn:acr:strong")
	legacy := NewLegacyChannelAuditInput(org, "subj-opaco", LegacyChannelSession{Channel: LegacyRADIUS})

	for _, in := range []AuditEventInput{sync, fed, legacy} {
		ev, err := NewAuditEvent(in)
		if err != nil {
			t.Fatalf("evento inbound deveria ser válido (%s): %v", in.Action, err)
		}
		if ev.SerializedOutcome() != "success" {
			t.Fatalf("evento %s deveria ser success", in.Action)
		}
	}

	// O evento de canal legado é sinalizado como legado.
	if !strings.Contains(legacy.Reason, "legacy:radius") {
		t.Fatalf("evento de canal legado deveria ser sinalizado: %q", legacy.Reason)
	}
	// O login federado registra o acr do IdP como informativo, em contexto L1.
	if fed.Context.AuthContextClass != "L1" || !strings.Contains(fed.Reason, "informativo") {
		t.Fatalf("login federado deveria ser L1 com acr informativo: %+v", fed)
	}
}

// As três ações são catalogadas (INV-8 as trata como isentas — verificado no
// suite de invariantes).
func TestDirectoryAuditActionsCatalogued(t *testing.T) {
	for _, a := range []Action{ActionDirectorySync, ActionFederatedLogin, ActionLegacyChannelAccess} {
		if !a.Valid() {
			t.Fatalf("ação %q deveria estar catalogada", a)
		}
	}
}
