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

func validActor() AuditActor {
	mid := uuid.New()
	sid := uuid.New()
	return AuditActor{IdentitySubject: "sub-opaque-123", MembershipID: &mid, SessionID: &sid}
}

func TestActionCatalogIsClosed(t *testing.T) {
	// Every catalogued action is valid and carries an assurance level.
	for _, a := range CatalogedActions() {
		if !a.Valid() {
			t.Errorf("ação catalogada %q não é válida", a)
		}
		if lvl := a.AssuranceLevel(); lvl != L1 && lvl != L2 && lvl != L3 {
			t.Errorf("ação %q sem nível de garantia definido: %v", a, lvl)
		}
	}
	// An action outside the catalog is not valid.
	if Action("totally.made.up").Valid() {
		t.Fatalf("ação fora do catálogo não deveria ser válida")
	}
	if ActionAuthLogin.AssuranceLevel() != L1 {
		t.Fatalf("auth.login deveria ser L1")
	}
	// A privileged verb is L3.
	if ActionPrivilegedSessionOpen.AssuranceLevel() != L3 {
		t.Fatalf("privileged.session.open deveria ser L3")
	}
}

// Cenário "Autenticação bem-sucedida": evento com ator, resultado e contexto.
func TestNewAuditEventSuccess(t *testing.T) {
	ev, err := NewAuditEvent(AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionAuthLogin,
		Actor:          validActor(),
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "identity", ID: "sub-opaque-123", Label: "login"},
		Reason:         "credenciais válidas + fator forte",
		Context:        AuditContext{IP: "203.0.113.7", UserAgent: "Mozilla/5.0", AuthContextClass: "L1", AuthMethods: []string{"pwd"}},
	})
	if err != nil {
		t.Fatalf("NewAuditEvent: %v", err)
	}
	if ev.SchemaVersion != AuditSchemaVersion {
		t.Fatalf("schema_version = %d, quero %d", ev.SchemaVersion, AuditSchemaVersion)
	}
	if ev.EventID.Version() != 7 {
		t.Fatalf("event_id não é UUIDv7: %s", ev.EventID)
	}
	if ev.SerializedOutcome() != "success" {
		t.Fatalf("outcome serializado = %q, quero success", ev.SerializedOutcome())
	}
	if ev.Action != ActionAuthLogin {
		t.Fatalf("action = %q", ev.Action)
	}
}

// Cenário "Autenticação negada": outcome denied com motivo.
func TestNewAuditEventDeniedAndError(t *testing.T) {
	denied, err := NewAuditEvent(AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionAuthLoginDenied,
		Actor:          AuditActor{IdentitySubject: "sub-x"},
		Outcome:        Denied,
		Reason:         "fator inválido",
	})
	if err != nil {
		t.Fatalf("NewAuditEvent denied: %v", err)
	}
	if denied.SerializedOutcome() != "denied" {
		t.Fatalf("outcome = %q, quero denied", denied.SerializedOutcome())
	}

	// Failed (controle indisponível) serializa como error (RFC-0003 §2).
	failed, err := NewAuditEvent(AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionAuthLoginDenied,
		Actor:          AuditActor{IdentitySubject: "sub-x"},
		Outcome:        Failed,
		Reason:         "auditoria indisponível",
	})
	if err != nil {
		t.Fatalf("NewAuditEvent failed: %v", err)
	}
	if failed.SerializedOutcome() != "error" {
		t.Fatalf("outcome = %q, quero error", failed.SerializedOutcome())
	}
}

func TestNewAuditEventRejectsInvalidInput(t *testing.T) {
	base := AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionAuthLogin,
		Actor:          validActor(),
		Outcome:        Allowed,
	}

	// Ação fora do catálogo.
	bad := base
	bad.Action = Action("nope.nope")
	if _, err := NewAuditEvent(bad); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("ação desconhecida: err = %v, quero ErrUnknownAction", err)
	}

	// Organização nula (a cadeia é por tenant — sem org não há evento).
	bad = base
	bad.OrganizationID = uuid.Nil
	if _, err := NewAuditEvent(bad); !errors.Is(err, ErrInvalidAuditEvent) {
		t.Fatalf("org nula: err = %v, quero ErrInvalidAuditEvent", err)
	}

	// Ator sem subject (não há evento sem ator identificável).
	bad = base
	bad.Actor = AuditActor{}
	if _, err := NewAuditEvent(bad); !errors.Is(err, ErrInvalidAuditEvent) {
		t.Fatalf("ator sem subject: err = %v, quero ErrInvalidAuditEvent", err)
	}

	// Outcome indefinido.
	bad = base
	bad.Outcome = Outcome(99)
	if _, err := NewAuditEvent(bad); !errors.Is(err, ErrInvalidAuditEvent) {
		t.Fatalf("outcome inválido: err = %v, quero ErrInvalidAuditEvent", err)
	}
}

// O conteúdo canônico NÃO carrega campos de cadeia — eles vivem no SealedEvent,
// atribuídos na escrita. Isto torna impossível, por construção, incluir o hash
// no próprio hash.
func TestSealedEventWrapsContent(t *testing.T) {
	ev, err := NewAuditEvent(AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionTenantSwitch,
		Actor:          validActor(),
		Outcome:        Allowed,
	})
	if err != nil {
		t.Fatalf("NewAuditEvent: %v", err)
	}
	sealed := SealedEvent{Event: ev, Seq: 42, PrevHash: []byte("prev"), Hash: []byte("h")}
	if sealed.Event.Action != ActionTenantSwitch || sealed.Seq != 42 {
		t.Fatalf("SealedEvent não compôs o conteúdo: %+v", sealed)
	}
	// The content type has no seq/hash fields to accidentally canonicalize.
	// (Compile-time guarantee: AuditEvent exposes none — this test documents it.)
}
