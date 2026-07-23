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
	"encoding/json"
	"errors"
	"testing"
)

func validClaims() OIDCClaims {
	return OIDCClaims{
		Issuer:        "https://archguard.example",
		Subject:       "sub-opaque",
		Audience:      "warpgate",
		ExpiresAt:     1700000900,
		IssuedAt:      1700000000,
		Organization:  "018f-org",
		MembershipID:  "018f-mid",
		ACR:           "L2",
		AMR:           []string{"pwd", "webauthn"},
		AuthTime:      1700000000,
		SessionID:     "018f-sid",
		ClaimsVersion: OIDCClaimsVersion,
	}
}

func TestOIDCClaimsWellFormed(t *testing.T) {
	if err := validClaims().WellFormed(); err != nil {
		t.Fatalf("claims válidos deveriam passar: %v", err)
	}
}

func TestOIDCClaimsWellFormedRejects(t *testing.T) {
	cases := map[string]func(*OIDCClaims){
		"sem iss":       func(c *OIDCClaims) { c.Issuer = "" },
		"sem sub":       func(c *OIDCClaims) { c.Subject = "" },
		"sem aud":       func(c *OIDCClaims) { c.Audience = "" },
		"sem org":       func(c *OIDCClaims) { c.Organization = "" },
		"sem mid":       func(c *OIDCClaims) { c.MembershipID = "" },
		"sem sid":       func(c *OIDCClaims) { c.SessionID = "" },
		"acr inválido":  func(c *OIDCClaims) { c.ACR = "L9" },
		"amr vazio":     func(c *OIDCClaims) { c.AMR = nil },
		"sem auth_time": func(c *OIDCClaims) { c.AuthTime = 0 },
		"exp <= iat":    func(c *OIDCClaims) { c.ExpiresAt = c.IssuedAt },
		"versão errada": func(c *OIDCClaims) { c.ClaimsVersion = "v0" },
	}
	for name, mangle := range cases {
		t.Run(name, func(t *testing.T) {
			c := validClaims()
			mangle(&c)
			if err := c.WellFormed(); !errors.Is(err, ErrInvalidClaims) {
				t.Fatalf("%s deveria ser inválido, err = %v", name, err)
			}
		})
	}
}

// O JSON serializado usa exatamente os nomes de claim do contrato (RFC-0006 §3);
// campos opcionais ausentes não aparecem; e-mail nunca vaza sem escopo.
func TestOIDCClaimsJSONContract(t *testing.T) {
	b, err := json.Marshal(validClaims())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, required := range []string{"iss", "sub", "aud", "exp", "iat", "org", "mid", "acr", "amr", "auth_time", "sid", "archguard_claims_version"} {
		if _, ok := m[required]; !ok {
			t.Fatalf("claim obrigatório %q ausente do JSON", required)
		}
	}
	// Sem escopo de e-mail: o claim email não aparece (I-3.2).
	if _, ok := m["email"]; ok {
		t.Fatalf("email não deveria aparecer sem escopo")
	}
	// Opcionais ausentes não poluem o token.
	for _, optional := range []string{"act", "pcid", "grant_ref", "groups", "roles"} {
		if _, ok := m[optional]; ok {
			t.Fatalf("claim opcional %q não deveria aparecer quando vazio", optional)
		}
	}
}
