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

const scimGroupBody = `{
  "schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
  "displayName": "DBA",
  "externalId": "grp-ext-7",
  "members": [{"value": "u1"}, {"value": "u2", "display": "Bob"}, {"value": ""}]
}`

func TestParseSCIMGroup(t *testing.T) {
	g, err := ParseSCIMGroup([]byte(scimGroupBody))
	if err != nil {
		t.Fatalf("ParseSCIMGroup: %v", err)
	}
	rec := g.ToGroupRecord()
	if rec.ExternalID != "grp-ext-7" || rec.DisplayName != "DBA" {
		t.Fatalf("mapeamento inesperado: %+v", rec)
	}
	if len(rec.MemberIDs) != 2 { // o membro com value vazio é ignorado
		t.Fatalf("esperava 2 membros, veio %d: %+v", len(rec.MemberIDs), rec.MemberIDs)
	}
}

func TestParseSCIMGroupValidations(t *testing.T) {
	if _, err := ParseSCIMGroup([]byte(`{`)); !errors.Is(err, ErrSCIMMalformed) {
		t.Fatalf("malformado deveria ser ErrSCIMMalformed, veio %v", err)
	}
	if _, err := ParseSCIMGroup([]byte(`{"schemas":["x"],"displayName":"DBA"}`)); !errors.Is(err, ErrSCIMGroupSchema) {
		t.Fatalf("schema errado deveria ser ErrSCIMGroupSchema, veio %v", err)
	}
	if _, err := ParseSCIMGroup([]byte(`{"schemas":["` + SCIMGroupSchema + `"]}`)); !errors.Is(err, ErrSCIMGroupDisplayName) {
		t.Fatalf("sem displayName deveria ser ErrSCIMGroupDisplayName, veio %v", err)
	}
}

func TestResponseGroupStampsMeta(t *testing.T) {
	g, _ := ParseSCIMGroup([]byte(scimGroupBody))
	resp := g.ResponseGroup("gid", "/scim/v2/Groups/gid")
	if resp.ID != "gid" || resp.Meta == nil || resp.Meta.ResourceType != "Group" {
		t.Fatalf("resposta sem id/meta: %+v", resp)
	}
}
