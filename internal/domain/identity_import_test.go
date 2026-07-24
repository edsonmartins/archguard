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

func TestImportRecordValidate(t *testing.T) {
	if err := (ImportRecord{Email: "a@b"}).Validate(); err != nil {
		t.Fatalf("com e-mail deveria validar: %v", err)
	}
	if err := (ImportRecord{DisplayName: "sem email"}).Validate(); !errors.Is(err, ErrImportEmailRequired) {
		t.Fatalf("sem e-mail deveria ser ErrImportEmailRequired, veio %v", err)
	}
}

func TestImportReportCount(t *testing.T) {
	r := ImportReport{BatchID: "b1", Entries: []ImportEntry{
		{Email: "a@b", Outcome: ImportCreated},
		{Email: "c@d", Outcome: ImportReused},
		{Email: "e@f", Outcome: ImportCreated},
		{Email: "bad", Outcome: ImportFailed},
	}}
	if r.Count(ImportCreated) != 2 || r.Count(ImportReused) != 1 || r.Count(ImportFailed) != 1 {
		t.Fatalf("contagens inesperadas: %+v", r)
	}
}

// O registro de importação NÃO tem campo de senha — por construção, nenhuma senha
// da origem pode ser importada (RFC-0007 §4).
func TestImportRecordHasNoPasswordField(t *testing.T) {
	rec := ImportRecord{Email: "a@b", DisplayName: "Ana", ExternalID: "x"}
	sync := rec.ToSyncRecord()
	if sync.Email != "a@b" || !sync.Active {
		t.Fatalf("registro neutro inesperado: %+v", sync)
	}
	// Se algum dia adicionarem um campo de senha, este teste + o ImportRecord
	// precisam ser reavaliados contra RFC-0007 §4.
}
