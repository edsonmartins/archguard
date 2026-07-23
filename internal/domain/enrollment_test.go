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

func TestRequiresEnrollment(t *testing.T) {
	id := uuid.New()
	pwd, _ := NewPasswordCredential(id, []byte("hash"), "pbkdf2", "salt")
	totp, _ := NewTOTPCredential(id, "ref")

	// Privilegiado sem fator forte (só senha) → exige enrolamento.
	if !RequiresEnrollment(true, []Credential{pwd}) {
		t.Fatalf("privilegiado só com senha deveria exigir enrolamento")
	}
	// Privilegiado com TOTP (forte) → não exige.
	if RequiresEnrollment(true, []Credential{pwd, totp}) {
		t.Fatalf("privilegiado com fator forte não deveria exigir enrolamento")
	}
	// Não-privilegiado → nunca exige aqui.
	if RequiresEnrollment(false, []Credential{pwd}) {
		t.Fatalf("não-privilegiado não deveria exigir enrolamento")
	}
	// Privilegiado sem nenhuma credencial → exige.
	if !RequiresEnrollment(true, nil) {
		t.Fatalf("privilegiado sem credenciais deveria exigir enrolamento")
	}
}

func enrollmentGuard(t *testing.T) *AssuranceGuard {
	t.Helper()
	cat := NewOperationCatalog()
	for _, op := range []Operation{
		{ID: "factor.enroll", Level: L1, AllowedDuringEnrollment: true},
		{ID: "profile.read", Level: L1},
	} {
		if err := cat.Register(op); err != nil {
			t.Fatalf("Register %s: %v", op.ID, err)
		}
	}
	return NewAssuranceGuard(cat)
}

// Estado bloqueante: uma sessão em enrolamento obrigatório só alcança operações
// de registro de fator; qualquer outra é recusada com ErrEnrollmentRequired.
func TestGuardBlocksDuringEnrollment(t *testing.T) {
	g := enrollmentGuard(t)
	s := activeSessionWithContext(t, AAL1, FactorPassword)
	s.MarkEnrollmentRequired()

	// Operação comum é bloqueada.
	if err := g.Authorize("profile.read", &s, AAL1, testNow); !errors.Is(err, ErrEnrollmentRequired) {
		t.Fatalf("operação comum em enrolamento: err = %v, quero ErrEnrollmentRequired", err)
	}
	// A operação de enrolamento é permitida.
	if err := g.Authorize("factor.enroll", &s, AAL1, testNow); err != nil {
		t.Fatalf("registro de fator deveria ser permitido em enrolamento: %v", err)
	}

	// Após enrolar (limpar o estado), a operação comum passa.
	s.ClearEnrollmentRequired()
	if err := g.Authorize("profile.read", &s, AAL1, testNow); err != nil {
		t.Fatalf("após enrolamento a operação comum deveria passar: %v", err)
	}
}

// O bloqueio precede a checagem de garantia: mesmo o enrolamento (L1) numa
// sessão obsoleta segue permitido, mas uma operação comum é bloqueada por
// enrolamento antes de qualquer avaliação de nível.
func TestGuardEnrollmentGatePrecedesAssurance(t *testing.T) {
	g := enrollmentGuard(t)
	s := activeSessionWithContext(t, AAL1, FactorPassword)
	s.MarkEnrollmentRequired()
	// Piso do tenant AAL3: ainda assim a recusa é por enrolamento, não por AAL.
	if err := g.Authorize("profile.read", &s, AAL3, testNow); !errors.Is(err, ErrEnrollmentRequired) {
		t.Fatalf("o gate de enrolamento deveria preceder o de garantia: %v", err)
	}
}
