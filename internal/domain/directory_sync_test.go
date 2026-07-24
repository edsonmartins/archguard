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

// Só os atributos MAPEADOS são projetados — nada fora do mapeamento explícito
// vaza, e a chave de dedup (email) sai do mapeamento.
func TestApplyAttributeMapping(t *testing.T) {
	mapping := []AttributeMapping{
		{DirectoryAttr: "mail", ArchGuardAttr: "email"},
		{DirectoryAttr: "displayName", ArchGuardAttr: "name"},
	}
	raw := map[string]string{
		"mail":            "ana@cli.com",
		"displayName":     "Ana",
		"telephoneNumber": "+55 11 99999-9999", // NÃO mapeado — não deve aparecer
		"department":      "",                  // mapeado? não; e vazio
	}
	out := ApplyAttributeMapping(mapping, raw)
	if out["email"] != "ana@cli.com" || out["name"] != "Ana" {
		t.Fatalf("atributos mapeados inesperados: %+v", out)
	}
	if _, ok := out["telephoneNumber"]; ok {
		t.Fatalf("atributo não mapeado não deveria vazar")
	}
	if MappedEmail(out) != "ana@cli.com" {
		t.Fatalf("MappedEmail deveria extrair a chave de dedup")
	}
	// Valor vazio na origem não cria entrada.
	if _, ok := ApplyAttributeMapping([]AttributeMapping{{DirectoryAttr: "x", ArchGuardAttr: "y"}}, map[string]string{"x": ""})["y"]; ok {
		t.Fatalf("valor vazio não deveria ser mapeado")
	}
}
