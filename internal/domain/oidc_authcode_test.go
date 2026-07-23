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
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func challengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestVerifyPKCE(t *testing.T) {
	verifier := "um-code-verifier-longo-e-aleatorio-de-teste-1234567890"
	if !VerifyPKCE(verifier, challengeFor(verifier)) {
		t.Fatalf("verifier correto deveria casar com o challenge")
	}
	if VerifyPKCE("outro", challengeFor(verifier)) {
		t.Fatalf("verifier errado não deveria casar")
	}
	if VerifyPKCE("", "") {
		t.Fatalf("vazios não deveriam casar (fail-closed)")
	}
}

// Ciclo do código: emitido -> resgatado com PKCE correto; expirado/usado/PKCE
// errado/redirect errado recusam.
func TestAuthorizationCodeRedeem(t *testing.T) {
	verifier := "verifier-de-teste-com-entropia-suficiente-abcdef123456"
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	secret, code, err := NewAuthorizationCode("warpgate", "https://wg/cb", challengeFor(verifier),
		uuid.New(), uuid.New(), []string{"openid"}, 60*time.Second, now)
	if err != nil {
		t.Fatalf("NewAuthorizationCode: %v", err)
	}
	if secret[:3] != "ac_" || len(code.CodeHash) == 0 {
		t.Fatalf("código malformado")
	}

	// Resgate correto.
	if err := code.Redeem("https://wg/cb", verifier, now.Add(time.Second)); err != nil {
		t.Fatalf("resgate correto deveria passar: %v", err)
	}
	// PKCE errado.
	if err := code.Redeem("https://wg/cb", "verifier-errado", now.Add(time.Second)); !errors.Is(err, ErrPKCEVerificationFailed) {
		t.Fatalf("PKCE errado: err = %v", err)
	}
	// redirect_uri errado.
	if err := code.Redeem("https://outro/cb", verifier, now.Add(time.Second)); !errors.Is(err, ErrRedirectURIMismatch) {
		t.Fatalf("redirect errado: err = %v", err)
	}
	// Expirado.
	if err := code.Redeem("https://wg/cb", verifier, now.Add(2*time.Minute)); !errors.Is(err, ErrAuthCodeExpiredOrUsed) {
		t.Fatalf("expirado: err = %v", err)
	}
	// Usado.
	used := code
	used.Used = true
	if err := used.Redeem("https://wg/cb", verifier, now.Add(time.Second)); !errors.Is(err, ErrAuthCodeExpiredOrUsed) {
		t.Fatalf("usado: err = %v", err)
	}
}
