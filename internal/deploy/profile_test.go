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

package deploy

import "testing"

func TestParseValid(t *testing.T) {
	for _, s := range []string{"dev", "pilot", "production"} {
		if p, err := Parse(s); err != nil || string(p) != s {
			t.Errorf("Parse(%q) = %v, %v", s, p, err)
		}
	}
}

func TestParseRejectsEmptyAndUnknown(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Error("perfil vazio deveria ser rejeitado (erro fatal de boot)")
	}
	if _, err := Parse("staging"); err == nil {
		t.Error("perfil desconhecido deveria ser rejeitado")
	}
}

func TestConformanceAndCustodian(t *testing.T) {
	if Dev.Conformant() {
		t.Error("dev NÃO é conforme")
	}
	if !Pilot.Conformant() || !Production.Conformant() {
		t.Error("pilot e production são conformes")
	}
	if Dev.KeyCustodian() != "local-sealed-keystore" {
		t.Errorf("custodiante do dev: %s", Dev.KeyCustodian())
	}
	if Production.KeyCustodian() != "openbao" {
		t.Errorf("custodiante do production: %s", Production.KeyCustodian())
	}
}
