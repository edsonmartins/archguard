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
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func subjectAuditEvent(t *testing.T, subject string, org uuid.UUID, occurred time.Time) AuditEvent {
	t.Helper()
	ev, err := NewAuditEvent(AuditEventInput{
		OrganizationID: org,
		Action:         ActionAuthLogin,
		Actor:          AuditActor{IdentitySubject: subject},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "session", ID: "s1", Label: "login"},
		Reason:         "login do titular",
	})
	if err != nil {
		t.Fatalf("NewAuditEvent: %v", err)
	}
	ev.OccurredAt = occurred
	return ev
}

// T-023 / spec "Integridade preservada": a eliminação de um titular (crypto-
// shredding) mantém a cadeia de auditoria verificável — a cadeia guarda só o
// pseudônimo, então destruir a chave de dados pessoais do titular é ORTOGONAL à
// integridade da cadeia.
func TestChainVerifiableAfterSubjectErasure(t *testing.T) {
	subject := "titular-opaco"
	org := uuid.New()
	base := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)

	// Cadeia de 2 eventos que referenciam o titular pelo pseudônimo.
	genesis := bytes.Repeat([]byte{0xA0}, 32)
	ev1 := subjectAuditEvent(t, subject, org, base)
	s1, err := SealEvent(ev1, genesis, 1)
	if err != nil {
		t.Fatalf("SealEvent 1: %v", err)
	}
	ev2 := subjectAuditEvent(t, subject, org, base.Add(time.Minute))
	s2, err := SealEvent(ev2, s1.Hash, 2)
	if err != nil {
		t.Fatalf("SealEvent 2: %v", err)
	}
	chain := []SealedEvent{s1, s2}

	if r := VerifyChain(genesis, chain); !r.OK {
		t.Fatalf("cadeia deveria verificar antes da eliminação: %+v", r)
	}

	// O conteúdo canônico do evento carrega o PSEUDÔNIMO, não dado pessoal.
	canon, _ := Canonical(ev1)
	if !strings.Contains(string(canon), subject) {
		t.Fatalf("o canônico deveria conter o pseudônimo do titular")
	}
	if strings.Contains(string(canon), "@") {
		t.Fatalf("o canônico NÃO deveria conter e-mail (dado pessoal): %s", canon)
	}

	// Elimina o titular (crypto-shredding da chave de dados pessoais).
	cipher := newFakeCipher()
	_, _ = cipher.EncryptForSubject(subject, []byte("ana@cli.com"))
	if err := cipher.DestroySubjectKey(subject); err != nil {
		t.Fatalf("DestroySubjectKey: %v", err)
	}

	// O verificador roda após a eliminação: a cadeia permanece ÍNTEGRA — os
	// eventos selados (só pseudônimo) não dependem da chave destruída.
	if r := VerifyChain(genesis, chain); !r.OK {
		t.Fatalf("cadeia deveria permanecer verificável após a eliminação: %+v", r)
	}
	// E o canônico do evento é idêntico ao de antes — a eliminação não o altera.
	canonAfter, _ := Canonical(ev1)
	if !bytes.Equal(canon, canonAfter) {
		t.Fatalf("a eliminação não deveria alterar o conteúdo canônico do evento")
	}
}
