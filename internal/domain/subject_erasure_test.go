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

// fakeCipher records destroyed subjects for the erasure test.
type fakeCipher struct{ destroyed map[string]bool }

func newFakeCipher() *fakeCipher { return &fakeCipher{destroyed: map[string]bool{}} }

func (c *fakeCipher) EncryptForSubject(id string, pt []byte) ([]byte, error) {
	if c.destroyed[id] {
		return nil, ErrSubjectKeyDestroyed
	}
	return pt, nil
}
func (c *fakeCipher) DecryptForSubject(id string, ct []byte) ([]byte, error) {
	if c.destroyed[id] {
		return nil, ErrSubjectKeyDestroyed
	}
	return ct, nil
}
func (c *fakeCipher) DestroySubjectKey(id string) error { c.destroyed[id] = true; return nil }

func erasureReq() ErasureRequest {
	return ErasureRequest{
		SubjectID: "titular-opaco", OrganizationID: uuid.New(),
		OperatorSubject: "admin-opaco", AcknowledgedIrreversible: true,
	}
}

// A eliminação exige confirmação explícita da irreversibilidade (spec
// "Confirmação da irreversibilidade").
func TestEraseSubjectRequiresAcknowledgement(t *testing.T) {
	c := newFakeCipher()
	req := erasureReq()
	req.AcknowledgedIrreversible = false
	if _, err := EraseSubject(c, req); !errors.Is(err, ErrErasureNotAcknowledged) {
		t.Fatalf("sem confirmação deveria falhar, veio %v", err)
	}
	if c.destroyed["titular-opaco"] {
		t.Fatalf("sem confirmação a chave NÃO deveria ter sido destruída")
	}
	if _, err := EraseSubject(c, ErasureRequest{AcknowledgedIrreversible: true}); !errors.Is(err, ErrErasureSubjectRequired) {
		t.Fatalf("sem titular deveria falhar, veio %v", err)
	}
}

// A eliminação destrói a chave e produz o evento de auditoria L3 com pseudônimos.
func TestEraseSubjectShredsAndAudits(t *testing.T) {
	c := newFakeCipher()
	req := erasureReq()
	in, err := EraseSubject(c, req)
	if err != nil {
		t.Fatalf("EraseSubject: %v", err)
	}
	if !c.destroyed["titular-opaco"] {
		t.Fatalf("a chave do titular deveria ter sido destruída")
	}
	if in.Action != ActionSubjectErasure || in.Context.AuthContextClass != "L3" {
		t.Fatalf("evento de auditoria inesperado: %+v", in)
	}
	// Só pseudônimos — nenhum dado pessoal reintroduzido pela auditoria.
	if in.Target.ID != "titular-opaco" || in.Actor.IdentitySubject != "admin-opaco" {
		t.Fatalf("auditoria deveria carregar só pseudônimos: %+v", in)
	}
	// A ação subject.erasure é L3 no catálogo.
	if ActionSubjectErasure.AssuranceLevel() != L3 {
		t.Fatalf("subject.erasure deveria ser L3")
	}
	ev, err := NewAuditEvent(in)
	if err != nil || ev.SerializedOutcome() != "success" {
		t.Fatalf("evento de eliminação deveria ser válido: %v", err)
	}
}
