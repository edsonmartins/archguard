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

package totp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

// fakeVault is an in-memory SecretStore for tests, with optional failure
// injection on Get to exercise the fail-closed path.
type fakeVault struct {
	secrets map[string][]byte
	puts    int
	getErr  error
}

func newFakeVault() *fakeVault { return &fakeVault{secrets: map[string][]byte{}} }

func (v *fakeVault) Put(_ context.Context, secret []byte) (string, error) {
	v.puts++
	ref := fmt.Sprintf("vault://totp/%d", v.puts)
	cp := make([]byte, len(secret))
	copy(cp, secret)
	v.secrets[ref] = cp
	return ref, nil
}

func (v *fakeVault) Get(_ context.Context, ref string) ([]byte, error) {
	if v.getErr != nil {
		return nil, v.getErr
	}
	s, ok := v.secrets[ref]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return s, nil
}

func (v *fakeVault) Delete(_ context.Context, ref string) error {
	delete(v.secrets, ref)
	return nil
}

func newTestService(t *testing.T, v domain.SecretStore) *Service {
	t.Helper()
	s, err := NewService("ArchGuard", v)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return s
}

// codeFor generates a currently-valid code for a pending enrollment's seed.
func codeFor(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	return code
}

// Ciclo completo: begin → confirma com código válido → credencial TOTP AAL2
// (forma INV-7: só SecretRef, semente no cofre) → Verify aceita código válido.
func TestEnrollAndVerify(t *testing.T) {
	vault := newFakeVault()
	svc := newTestService(t, vault)
	idID := uuid.New()

	enr, err := svc.BeginEnrollment(idID, "sub-totp")
	if err != nil {
		t.Fatalf("BeginEnrollment: %v", err)
	}
	// Enquanto não confirmado, nada foi ao cofre.
	if vault.puts != 0 {
		t.Fatalf("semente não confirmada não deveria ir ao cofre")
	}

	cred, err := svc.FinishEnrollment(context.Background(), enr, codeFor(t, enr.secret))
	if err != nil {
		t.Fatalf("FinishEnrollment: %v", err)
	}
	if cred.Type != domain.FactorTOTP || !cred.WellFormed() {
		t.Fatalf("credencial TOTP malformada: %+v", cred)
	}
	if cred.AAL != domain.AAL2 {
		t.Fatalf("TOTP deveria ser AAL2, veio %s", cred.AAL)
	}
	// INV-7: só referência de cofre — nada de verifier nem material público, e a
	// semente não está na credencial.
	if cred.SecretRef == "" || len(cred.Verifier) != 0 || len(cred.PublicMaterial) != 0 {
		t.Fatalf("forma INV-7 violada: %+v", cred)
	}
	if vault.puts != 1 {
		t.Fatalf("semente confirmada deveria ir ao cofre exatamente uma vez, foram %d", vault.puts)
	}

	ok, err := svc.Verify(context.Background(), cred, codeFor(t, enr.secret))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatalf("código válido deveria ser aceito")
	}

	// Código errado é negação (ok=false), não erro.
	ok, err = svc.Verify(context.Background(), cred, "000000")
	if err != nil {
		t.Fatalf("código errado não deveria ser erro: %v", err)
	}
	if ok {
		t.Fatalf("código errado não deveria ser aceito")
	}
}

// Confirmação com código errado é recusada e NADA é custodiado — sem semente
// órfã no cofre.
func TestFinishRejectsWrongCode(t *testing.T) {
	vault := newFakeVault()
	svc := newTestService(t, vault)
	enr, err := svc.BeginEnrollment(uuid.New(), "sub-x")
	if err != nil {
		t.Fatalf("BeginEnrollment: %v", err)
	}
	if _, err := svc.FinishEnrollment(context.Background(), enr, "000000"); err == nil {
		t.Fatalf("código de confirmação errado deveria ser recusado")
	}
	if vault.puts != 0 {
		t.Fatalf("semente não confirmada não deveria ser custodiada")
	}
}

// Restrição de nível: um TOTP recém-registrado é AAL2, não resiste a phishing e
// NÃO pode ser promovido a AAL3 — logo nunca satisfaz um step-up L3 (ADR-0010).
func TestTOTPCannotSatisfyL3(t *testing.T) {
	vault := newFakeVault()
	svc := newTestService(t, vault)
	enr, _ := svc.BeginEnrollment(uuid.New(), "sub-l3")
	cred, err := svc.FinishEnrollment(context.Background(), enr, codeFor(t, enr.secret))
	if err != nil {
		t.Fatalf("FinishEnrollment: %v", err)
	}
	if cred.PhishingResistant() {
		t.Fatalf("TOTP não deveria ser phishing-resistant")
	}
	if err := cred.SetAssurance(domain.AAL3); !errors.Is(err, domain.ErrAssuranceExceedsCeiling) {
		t.Fatalf("TOTP promovido a AAL3: err = %v, quero ErrAssuranceExceedsCeiling", err)
	}
}

// Fail-closed (INV-6): falha do cofre na verificação é ERRO, nunca uma negação
// silenciosa que um chamador poderia confundir com "código errado".
func TestVerifyVaultFailureIsError(t *testing.T) {
	vault := newFakeVault()
	svc := newTestService(t, vault)
	enr, _ := svc.BeginEnrollment(uuid.New(), "sub-v")
	cred, err := svc.FinishEnrollment(context.Background(), enr, codeFor(t, enr.secret))
	if err != nil {
		t.Fatalf("FinishEnrollment: %v", err)
	}
	vault.getErr = errors.New("cofre indisponível")
	ok, err := svc.Verify(context.Background(), cred, codeFor(t, enr.secret))
	if err == nil {
		t.Fatalf("falha do cofre deveria ser erro (fail-closed)")
	}
	if ok {
		t.Fatalf("falha do cofre não deveria autenticar")
	}
}

// SMS não é um tipo de fator — a configuração "SMS como fator" é estruturalmente
// impossível (spec: "SMS como fator → rejeitado"). Não há FactorType nem
// construtor de SMS.
func TestSMSIsNotAValidFactor(t *testing.T) {
	if domain.FactorType("sms").Valid() {
		t.Fatalf("SMS não deveria ser um tipo de fator válido")
	}
}
