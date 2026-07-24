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

import "testing"

// REGRA DURA (RFC-0007 §5.3 / T-013): nenhum acr de terceiro satisfaz um step-up
// L3. Uma sessão federada nunca é phishing-resistant nem passa de AAL1, então
// L3.Satisfies é falso para QUALQUER acr declarado pelo IdP.
func TestFederatedNeverSatisfiesL3(t *testing.T) {
	strongACRs := []string{
		"", "urn:acr:strong",
		"http://schemas.microsoft.com/claims/multipleauthn",
		"phr", "phrh", "http://id.example/loa/3",
	}
	for _, acr := range strongACRs {
		f := FederatedIdentity{Provider: "entra", Protocol: FederationOIDC, Email: "ana@cli.com", IdPACR: acr}

		if L3.Satisfies(f.ProvenAAL(), f.PhishingResistant()) {
			t.Fatalf("acr %q não deveria satisfazer L3 por federação", acr)
		}
		if f.AuthorizesL3() {
			t.Fatalf("acr %q: AuthorizesL3 deveria ser false", acr)
		}
		// L2 também exige step-up do ArchGuard (federação = AAL1).
		if L2.Satisfies(f.ProvenAAL(), f.PhishingResistant()) {
			t.Fatalf("acr %q: L2 deveria exigir step-up do ArchGuard, não vir do IdP", acr)
		}
	}
}

// A federação estabelece identificação (L1) — o login federado é aceito como
// autenticação de base, e o step-up para L2/L3 é do ArchGuard.
func TestFederatedSatisfiesL1(t *testing.T) {
	f := FederatedIdentity{Provider: "okta", Protocol: FederationSAML, Email: "ana@cli.com"}
	if !L1.Satisfies(f.ProvenAAL(), f.PhishingResistant()) {
		t.Fatalf("federação deveria estabelecer identificação (L1)")
	}
	if f.ProvenAAL() != AAL1 || f.PhishingResistant() {
		t.Fatalf("federação deveria ser AAL1 e NÃO phishing-resistant: %v %v", f.ProvenAAL(), f.PhishingResistant())
	}
}
