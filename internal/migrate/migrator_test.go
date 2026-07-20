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

package migrate

import (
	"testing"
	"testing/fstest"
)

func TestParseMigrationsOrdersAndValidates(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0002_b.sql": {Data: []byte("SELECT 2;")},
		"migrations/0001_a.sql": {Data: []byte("SELECT 1;")},
		"migrations/0010_c.sql": {Data: []byte("SELECT 10;")},
	}
	ms, err := parseMigrations(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 || ms[0].version != 1 || ms[1].version != 2 || ms[2].version != 10 {
		t.Fatalf("ordenação incorreta: %+v", ms)
	}
	if ms[0].name != "a" || ms[0].sql != "SELECT 1;" {
		t.Fatalf("parse incorreto: %+v", ms[0])
	}
}

func TestParseMigrationsRejectsBadName(t *testing.T) {
	fsys := fstest.MapFS{"migrations/drop-stuff.sql": {Data: []byte("x")}}
	if _, err := parseMigrations(fsys); err == nil {
		t.Fatal("nome inválido deveria ser rejeitado")
	}
}

func TestParseMigrationsRejectsDuplicateVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0001_a.sql": {Data: []byte("x")},
		"migrations/0001_b.sql": {Data: []byte("y")},
	}
	if _, err := parseMigrations(fsys); err == nil {
		t.Fatal("versão duplicada deveria ser rejeitada")
	}
}

func TestPendingSkipsApplied(t *testing.T) {
	all := []migration{{version: 1}, {version: 2}, {version: 3}}
	if got := pending(all, 2); len(got) != 1 || got[0].version != 3 {
		t.Fatalf("pending(applied=2) deveria retornar só a 3: %+v", got)
	}
	if got := pending(all, 0); len(got) != 3 {
		t.Fatalf("pending(applied=0) deveria retornar todas: %+v", got)
	}
	if got := pending(all, 3); len(got) != 0 {
		t.Fatalf("pending(applied=3) deveria ser vazio: %+v", got)
	}
}

func TestEmbeddedMigrationsAreValid(t *testing.T) {
	// A suíte real garante que os arquivos embutidos parseiam e ordenam.
	ms, err := parseMigrations(migrationsFS)
	if err != nil {
		t.Fatalf("migrations embutidas inválidas: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("nenhuma migration embutida encontrada")
	}
}

func TestDsnRedacted(t *testing.T) {
	got := dsnRedacted("user=postgres password=secret host=localhost")
	if got != "user=postgres password=*** host=localhost" {
		t.Fatalf("senha não redigida: %q", got)
	}
}
