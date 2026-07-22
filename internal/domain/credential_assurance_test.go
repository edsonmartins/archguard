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

func TestFactorMaxAAL(t *testing.T) {
	cases := map[FactorType]AAL{
		FactorWebAuthn:     AAL3,
		FactorTOTP:         AAL2,
		FactorRecoveryCode: AAL2,
		FactorPassword:     AAL1,
	}
	for ft, want := range cases {
		if got := MaxAAL(ft); got != want {
			t.Errorf("MaxAAL(%s) = %s, quero %s", ft, got, want)
		}
	}
}

func TestPhishingResistantAndStrong(t *testing.T) {
	id := uuid.New()
	web, _ := NewWebAuthnCredential(id, []byte("pub"))
	totp, _ := NewTOTPCredential(id, "ref")
	pwd, _ := NewPasswordCredential(id, []byte("hash"), "pbkdf2", "salt")
	rec, _ := NewRecoveryCodeCredential(id, []byte("codehash"))

	// Phishing-resistant: só WebAuthn.
	if !web.PhishingResistant() {
		t.Fatalf("WebAuthn deveria ser phishing-resistant")
	}
	for _, c := range []Credential{totp, pwd, rec} {
		if c.PhishingResistant() {
			t.Fatalf("%s não deveria ser phishing-resistant", c.Type)
		}
	}

	// Strong: WebAuthn e TOTP.
	if !web.Strong() || !totp.Strong() {
		t.Fatalf("WebAuthn e TOTP deveriam ser fortes")
	}
	if pwd.Strong() || rec.Strong() {
		t.Fatalf("senha e recovery não deveriam contar como fator forte")
	}
}

// O teto por tipo: WebAuthn sobe a AAL3; TOTP/recovery no máximo AAL2; senha AAL1.
func TestSetAssuranceCeiling(t *testing.T) {
	id := uuid.New()

	web, _ := NewWebAuthnCredential(id, []byte("pub"))
	if err := web.SetAssurance(AAL3); err != nil {
		t.Fatalf("WebAuthn deveria poder ir a AAL3: %v", err)
	}
	if web.AAL != AAL3 || !web.WellFormed() {
		t.Fatalf("WebAuthn AAL3 deveria ser WellFormed: %+v", web)
	}

	totp, _ := NewTOTPCredential(id, "ref")
	if err := totp.SetAssurance(AAL3); !errors.Is(err, ErrAssuranceExceedsCeiling) {
		t.Fatalf("TOTP AAL3: err = %v, quero ErrAssuranceExceedsCeiling", err)
	}
	if err := totp.SetAssurance(AAL2); err != nil {
		t.Fatalf("TOTP deveria poder ficar em AAL2: %v", err)
	}

	pwd, _ := NewPasswordCredential(id, []byte("hash"), "pbkdf2", "salt")
	if err := pwd.SetAssurance(AAL2); !errors.Is(err, ErrAssuranceExceedsCeiling) {
		t.Fatalf("senha AAL2: err = %v, quero ErrAssuranceExceedsCeiling", err)
	}
}

// Um TOTP com AAL3 (forjado) NÃO é WellFormed — a base do "TOTP recusado em L3".
func TestWellFormedRejectsAssuranceAboveCeiling(t *testing.T) {
	id := uuid.New()
	totp, _ := NewTOTPCredential(id, "ref")
	totp.AAL = AAL3 // forja direta, contornando SetAssurance
	if totp.WellFormed() {
		t.Fatalf("TOTP alegando AAL3 não deveria ser WellFormed")
	}
}
