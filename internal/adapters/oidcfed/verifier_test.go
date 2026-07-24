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

package oidcfed

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

type fakeTokens struct {
	claims IDTokenClaims
	err    error
}

func (f fakeTokens) VerifyIDToken(string) (IDTokenClaims, error) { return f.claims, f.err }

func fixedNow() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) }

func goodClaims() IDTokenClaims {
	return IDTokenClaims{
		Issuer: "https://op.cli.com", Subject: "sub-1", Audience: Audience{"archguard-client"},
		Expiry: fixedNow().Add(time.Hour).Unix(), Email: "ana@cli.com", EmailVerified: true,
		Name: "Ana", Nonce: "n-123", ACR: "urn:acr:strong",
	}
}

func newVerifier(tokens TokenVerifier) *Verifier {
	return NewVerifier(tokens, "https://op.cli.com", "archguard-client", "okta", fixedNow)
}

func TestVerifyOKMapsAndNeverL3(t *testing.T) {
	fed, err := newVerifier(fakeTokens{claims: goodClaims()}).Verify("<jwt>", "n-123")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if fed.Email != "ana@cli.com" || fed.ExternalID != "sub-1" || fed.Protocol != domain.FederationOIDC {
		t.Fatalf("mapeamento inesperado: %+v", fed)
	}
	if fed.IdPACR != "urn:acr:strong" || fed.AuthorizesL3() {
		t.Fatalf("acr do OP é informativo e não autoriza L3")
	}
}

func TestVerifyFailClosedClaims(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*IDTokenClaims)
		nonce string
		want  error
	}{
		{"issuer errado", func(c *IDTokenClaims) { c.Issuer = "https://mau.com" }, "n-123", ErrIssuer},
		{"audiência errada", func(c *IDTokenClaims) { c.Audience = Audience{"outro"} }, "n-123", ErrAudience},
		{"expirado", func(c *IDTokenClaims) { c.Expiry = fixedNow().Add(-time.Minute).Unix() }, "n-123", ErrExpired},
		{"nonce diferente", func(c *IDTokenClaims) { c.Nonce = "outro" }, "n-123", ErrNonce},
		{"email não verificado", func(c *IDTokenClaims) { c.EmailVerified = false }, "n-123", ErrEmailUnverified},
	}
	for _, c := range cases {
		claims := goodClaims()
		c.mut(&claims)
		_, err := newVerifier(fakeTokens{claims: claims}).Verify("<jwt>", c.nonce)
		if !errors.Is(err, c.want) {
			t.Fatalf("%s: esperava %v, veio %v", c.name, c.want, err)
		}
	}
}

func TestVerifySignatureError(t *testing.T) {
	_, err := newVerifier(fakeTokens{err: errors.New("assinatura inválida")}).Verify("<jwt>", "")
	if err == nil {
		t.Fatalf("assinatura inválida deveria falhar")
	}
}

// aud aceita string única e array (OIDC).
func TestAudienceUnmarshal(t *testing.T) {
	var a Audience
	if err := json.Unmarshal([]byte(`"c1"`), &a); err != nil || !a.Contains("c1") {
		t.Fatalf("aud string: %v %+v", err, a)
	}
	var b Audience
	if err := json.Unmarshal([]byte(`["c1","c2"]`), &b); err != nil || !b.Contains("c2") {
		t.Fatalf("aud array: %v %+v", err, b)
	}
}
