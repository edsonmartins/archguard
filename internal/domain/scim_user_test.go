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

const scimUserBody = `{
  "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
  "userName": "ana",
  "externalId": "idp-ext-42",
  "name": {"givenName": "Ana", "familyName": "Souza"},
  "emails": [{"value": "old@cli.com"}, {"value": "ana@cli.com", "primary": true}],
  "active": true,
  "password": "NAO-DEVE-SER-LIDA"
}`

func TestParseSCIMUser(t *testing.T) {
	u, err := ParseSCIMUser([]byte(scimUserBody))
	if err != nil {
		t.Fatalf("ParseSCIMUser: %v", err)
	}
	if u.PrimaryEmail() != "ana@cli.com" {
		t.Fatalf("e-mail primário inesperado: %q", u.PrimaryEmail())
	}
	if u.DisplayName() != "Ana Souza" {
		t.Fatalf("display name inesperado: %q", u.DisplayName())
	}
	rec := u.ToSyncRecord()
	if rec.ExternalID != "idp-ext-42" || rec.Email != "ana@cli.com" || !rec.Active {
		t.Fatalf("mapeamento p/ registro neutro inesperado: %+v", rec)
	}
	if rec.Attributes["name"] != "Ana Souza" {
		t.Fatalf("atributo name não mapeado: %+v", rec.Attributes)
	}
}

func TestParseSCIMUserValidations(t *testing.T) {
	if _, err := ParseSCIMUser([]byte(`{`)); !errors.Is(err, ErrSCIMMalformed) {
		t.Fatalf("corpo malformado deveria ser ErrSCIMMalformed, veio %v", err)
	}
	if _, err := ParseSCIMUser([]byte(`{"schemas":["x"],"userName":"a","emails":[{"value":"a@b"}]}`)); !errors.Is(err, ErrSCIMSchema) {
		t.Fatalf("schema errado deveria ser ErrSCIMSchema, veio %v", err)
	}
	if _, err := ParseSCIMUser([]byte(`{"schemas":["` + SCIMUserSchema + `"],"emails":[{"value":"a@b"}]}`)); !errors.Is(err, ErrSCIMUserName) {
		t.Fatalf("sem userName deveria ser ErrSCIMUserName, veio %v", err)
	}
	if _, err := ParseSCIMUser([]byte(`{"schemas":["` + SCIMUserSchema + `"],"userName":"a"}`)); !errors.Is(err, ErrSCIMEmailRequired) {
		t.Fatalf("sem e-mail deveria ser ErrSCIMEmailRequired, veio %v", err)
	}
}

// Nenhuma senha é lida do SCIM (RFC-0007 §4): o campo password é ignorado.
func TestSCIMUserIgnoresPassword(t *testing.T) {
	u, err := ParseSCIMUser([]byte(scimUserBody))
	if err != nil {
		t.Fatalf("ParseSCIMUser: %v", err)
	}
	// SCIMUser não tem campo de senha — nada a vazar. O registro neutro também não.
	rec := u.ToSyncRecord()
	for k, v := range rec.Attributes {
		if v == "NAO-DEVE-SER-LIDA" {
			t.Fatalf("senha vazou no atributo %q", k)
		}
	}
}

func TestResponseUserStampsMeta(t *testing.T) {
	u, _ := ParseSCIMUser([]byte(scimUserBody))
	resp := u.ResponseUser("id-atribuido", "/scim/v2/Users/id-atribuido")
	if resp.ID != "id-atribuido" || resp.Meta == nil || resp.Meta.ResourceType != "User" {
		t.Fatalf("resposta SCIM sem id/meta corretos: %+v", resp)
	}
}
