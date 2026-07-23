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
)

// PKCE ausente ou "plain" no Authorization Code é recusado (cenário "PKCE
// ausente").
func TestPKCEMandatory(t *testing.T) {
	if err := ValidateAuthorizationCodeRequest("code", "", "S256"); !errors.Is(err, ErrPKCERequired) {
		t.Fatalf("sem code_challenge deveria recusar: %v", err)
	}
	if err := ValidateAuthorizationCodeRequest("code", "abc", "plain"); !errors.Is(err, ErrPKCERequired) {
		t.Fatalf("método plain deveria recusar: %v", err)
	}
	if err := ValidateAuthorizationCodeRequest("code", "abc", "S256"); err != nil {
		t.Fatalf("Authorization Code com PKCE S256 deveria passar: %v", err)
	}
}

// Implicit (response_type token) e ROPC (grant_type password) são recusados
// (cenário "Fluxo obsoleto").
func TestObsoleteFlowsRejected(t *testing.T) {
	if err := ValidateResponseType("token"); !errors.Is(err, ErrUnsupportedFlow) {
		t.Fatalf("implicit deveria ser recusado: %v", err)
	}
	if err := ValidateResponseType("id_token token"); !errors.Is(err, ErrUnsupportedFlow) {
		t.Fatalf("hybrid com token deveria ser recusado: %v", err)
	}
	if err := ValidateGrantType("password"); !errors.Is(err, ErrUnsupportedFlow) {
		t.Fatalf("ROPC deveria ser recusado: %v", err)
	}
	if err := ValidateGrantType("client_credentials"); !errors.Is(err, ErrUnsupportedFlow) {
		t.Fatalf("grant não suportado deveria ser recusado: %v", err)
	}
	// Suportados.
	for _, g := range []string{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:device_code"} {
		if err := ValidateGrantType(g); err != nil {
			t.Fatalf("grant %q deveria ser suportado: %v", g, err)
		}
	}
	if err := ValidateResponseType("code"); err != nil {
		t.Fatalf("response_type code deveria ser suportado: %v", err)
	}
}
