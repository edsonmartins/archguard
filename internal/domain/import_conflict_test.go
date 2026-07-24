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

// Conflito de dedup no lote: o mesmo e-mail (mesmo com case/espaço diferentes)
// aparece duas vezes -> conflito para revisão humana, nunca fusão automática.
func TestDetectImportConflicts(t *testing.T) {
	records := []ImportRecord{
		{Email: "ana@cli.com", DisplayName: "Ana A"},
		{Email: "  ANA@cli.com ", DisplayName: "Ana B"}, // mesma pessoa, registro divergente
		{Email: "bob@cli.com", DisplayName: "Bob"},
		{Email: "", DisplayName: "sem email"}, // não é conflito (é falha de validação)
	}
	conflicts := DetectImportConflicts(records)
	if len(conflicts) != 1 {
		t.Fatalf("esperava 1 conflito (ana duplicada), veio %d: %+v", len(conflicts), conflicts)
	}
	if conflicts[0].Kind != ImportConflictIntraBatchDuplicate {
		t.Fatalf("tipo de conflito inesperado: %s", conflicts[0].Kind)
	}
	if !ConflictedEmails(conflicts)[NormalizeEmail("ana@cli.com")] {
		t.Fatalf("o e-mail em conflito deveria ser marcado para pular a importação")
	}
	// Bob (único) não está em conflito.
	if ConflictedEmails(conflicts)[NormalizeEmail("bob@cli.com")] {
		t.Fatalf("e-mail único não deveria estar em conflito")
	}
}

func TestDetectImportConflictsNone(t *testing.T) {
	records := []ImportRecord{{Email: "a@x"}, {Email: "b@x"}, {Email: "c@x"}}
	if c := DetectImportConflicts(records); len(c) != 0 {
		t.Fatalf("e-mails distintos não deveriam gerar conflito: %+v", c)
	}
}
