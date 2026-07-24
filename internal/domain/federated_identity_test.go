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

// REGRA DURA (RFC-0007 §5.3): o acr do IdP externo é informativo e NUNCA autoriza
// uma operação L3 — nem com acr máximo declarado.
func TestFederatedIdentityNeverAuthorizesL3(t *testing.T) {
	f := FederatedIdentity{
		Provider: "entra", Protocol: FederationSAML, Email: "ana@cli.com",
		IdPACR: "http://schemas.microsoft.com/claims/multipleauthn", // acr "forte" do IdP
	}
	if f.AuthorizesL3() {
		t.Fatalf("nenhum acr de terceiro pode autorizar L3 no ArchGuard")
	}
}

func TestFederatedIdentityValidate(t *testing.T) {
	if err := (FederatedIdentity{Email: "a@b"}).Validate(); err != nil {
		t.Fatalf("com e-mail deveria validar: %v", err)
	}
	if err := (FederatedIdentity{}).Validate(); !errors.Is(err, ErrFederatedEmailRequired) {
		t.Fatalf("sem e-mail deveria ser ErrFederatedEmailRequired, veio %v", err)
	}
}

// JIT via o mesmo caminho neutro de dedup (email é a chave).
func TestFederatedIdentityToSyncRecord(t *testing.T) {
	f := FederatedIdentity{Provider: "okta", Protocol: FederationOIDC, ExternalID: "sub-1",
		Email: "ana@cli.com", DisplayName: "Ana"}
	rec := f.ToSyncRecord()
	if rec.Email != "ana@cli.com" || rec.ExternalID != "sub-1" || !rec.Active {
		t.Fatalf("registro neutro inesperado: %+v", rec)
	}
	if rec.Attributes["name"] != "Ana" {
		t.Fatalf("display name não mapeado: %+v", rec.Attributes)
	}
}
