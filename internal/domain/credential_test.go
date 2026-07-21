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
	"errors"
	"testing"

	"github.com/google/uuid"
)

func credID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestNewPasswordCredential(t *testing.T) {
	idn := credID(t)
	c, err := NewPasswordCredential(idn, []byte("hash"), "bcrypt", "salt")
	if err != nil {
		t.Fatalf("NewPasswordCredential: %v", err)
	}
	if c.Type != FactorPassword || c.AAL != AAL1 {
		t.Errorf("type/aal = %q/%q, quer password/aal1", c.Type, c.AAL)
	}
	if !bytes.Equal(c.Verifier, []byte("hash")) {
		t.Error("verifier não preservado")
	}
	if c.Params["algo"] != "bcrypt" || c.Params["salt"] != "salt" {
		t.Errorf("params = %v", c.Params)
	}
	if !c.WellFormed() {
		t.Error("credencial de senha deveria ser well-formed")
	}
	if c.SecretRef != "" || len(c.PublicMaterial) != 0 {
		t.Error("senha não deveria ter secret_ref nem material público")
	}
}

func TestNewTOTPCredentialHoldsOnlyRef(t *testing.T) {
	idn := credID(t)
	c, err := NewTOTPCredential(idn, "vault://ref-123")
	if err != nil {
		t.Fatalf("NewTOTPCredential: %v", err)
	}
	if c.Type != FactorTOTP || c.AAL != AAL2 {
		t.Errorf("type/aal = %q/%q, quer totp/aal2", c.Type, c.AAL)
	}
	if c.SecretRef != "vault://ref-123" {
		t.Errorf("secretRef = %q", c.SecretRef)
	}
	// INV-7: um TOTP NUNCA carrega o seed — só a referência.
	if len(c.Verifier) != 0 || len(c.PublicMaterial) != 0 {
		t.Error("TOTP não pode carregar verifier nem material — só secret_ref")
	}
	if !c.WellFormed() {
		t.Error("TOTP com ref deveria ser well-formed")
	}
}

func TestNewWebAuthnAndRecoveryCredential(t *testing.T) {
	idn := credID(t)
	wa, err := NewWebAuthnCredential(idn, []byte("public-key-blob"))
	if err != nil || wa.Type != FactorWebAuthn || !wa.WellFormed() {
		t.Fatalf("WebAuthn: err=%v type=%q wf=%v", err, wa.Type, wa.WellFormed())
	}
	rc, err := NewRecoveryCodeCredential(idn, []byte("code-hash"))
	if err != nil || rc.Type != FactorRecoveryCode || !rc.WellFormed() {
		t.Fatalf("recovery: err=%v type=%q wf=%v", err, rc.Type, rc.WellFormed())
	}
}

func TestCredentialConstructorsRejectEmptyAndNilIdentity(t *testing.T) {
	idn := credID(t)
	if _, err := NewPasswordCredential(idn, nil, "bcrypt", ""); !errors.Is(err, ErrInvalidCredential) {
		t.Error("verifier vazio deveria ser rejeitado")
	}
	if _, err := NewTOTPCredential(idn, ""); !errors.Is(err, ErrInvalidCredential) {
		t.Error("secretRef vazio deveria ser rejeitado")
	}
	if _, err := NewWebAuthnCredential(idn, nil); !errors.Is(err, ErrInvalidCredential) {
		t.Error("material vazio deveria ser rejeitado")
	}
	if _, err := NewPasswordCredential(uuid.Nil, []byte("h"), "bcrypt", ""); !errors.Is(err, ErrInvalidCredential) {
		t.Error("identidade nula deveria ser rejeitada")
	}
}

func TestWellFormedRejectsMixedMaterial(t *testing.T) {
	idn := credID(t)
	// Um TOTP que (indevidamente) carregasse um verifier NÃO é well-formed —
	// esta é a barreira estrutural do INV-7 na camada de domínio.
	bad := Credential{ID: credID(t), IdentityID: idn, Type: FactorTOTP, AAL: AAL2,
		SecretRef: "ref", Verifier: []byte("seed-em-claro")}
	if bad.WellFormed() {
		t.Error("TOTP com verifier preenchido não pode ser well-formed (INV-7)")
	}
	// Senha sem verifier não é well-formed.
	bad2 := Credential{ID: credID(t), IdentityID: idn, Type: FactorPassword, AAL: AAL1}
	if bad2.WellFormed() {
		t.Error("senha sem verifier não pode ser well-formed")
	}
	// AAL inválido invalida.
	bad3 := Credential{ID: credID(t), IdentityID: idn, Type: FactorPassword, AAL: "aal9", Verifier: []byte("h")}
	if bad3.WellFormed() {
		t.Error("AAL inválido não pode ser well-formed")
	}
}

func TestFactorTypeAndAALValid(t *testing.T) {
	for _, ty := range []FactorType{FactorPassword, FactorTOTP, FactorWebAuthn, FactorRecoveryCode} {
		if !ty.Valid() {
			t.Errorf("%q deveria ser válido", ty)
		}
	}
	if FactorType("sms").Valid() {
		t.Error("sms não deveria ser válido")
	}
	for _, a := range []AAL{AAL1, AAL2, AAL3} {
		if !a.Valid() {
			t.Errorf("%q deveria ser válido", a)
		}
	}
	if AAL("aal0").Valid() {
		t.Error("aal0 não deveria ser válido")
	}
}
