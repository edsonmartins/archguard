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

package invariants

// "Teste do teste" (gate do pacote 001): cada detector precisa acusar uma
// violação injetada. Se um destes testes falhar, a suíte perdeu a capacidade
// de detecção e NÃO protege mais os cherry-picks — trate como incidente.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfINV1DetectsInjectedViolation(t *testing.T) {
	root := filepath.Join(repoRoot(t), "test", "invariants", "testdata", "inv1")
	found := findMasterCredential(t, root, []string{"bad_auth.go"})
	if len(found) != 1 {
		t.Fatalf("detector INV-1 não acusou a violação injetada em testdata/inv1 (found=%v)", found)
	}
	if vs := checkINV1(found, nil); len(vs) != 1 {
		t.Fatalf("checkINV1 deveria reportar 1 violação sem allowlist, reportou %d", len(vs))
	}
}

func TestSelfINV1AllowlistRejectsGlob(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "known_violations.txt")
	if err := os.WriteFile(p, []byte("INV-1 object/*.go:MasterPassword\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseAllowlist(p); err == nil || !strings.Contains(err.Error(), "proibidos") {
		t.Fatalf("allowlist com glob deveria ser rejeitada, err=%v", err)
	}
}

func TestSelfINV1AllowlistFailsWhenStale(t *testing.T) {
	entries := []allowlistEntry{{File: "object/gone.go", Symbol: "MasterPassword"}}
	vs := checkINV1(nil, entries)
	if len(vs) != 1 || !strings.Contains(vs[0], "obsoleta") {
		t.Fatalf("entrada obsoleta deveria falhar a suíte, vs=%v", vs)
	}
}

func TestSelfINV1AllowlistFailsWhenEmptyFileExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "known_violations.txt")
	if err := os.WriteFile(p, []byte("# só comentários\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseAllowlist(p); err == nil || !strings.Contains(err.Error(), "T-011") {
		t.Fatalf("arquivo existente sem entradas deveria falhar (autodestruição pós-T-011), err=%v", err)
	}
}

func TestSelfINV2DetectsInjectedMutation(t *testing.T) {
	root := filepath.Join(repoRoot(t), "test", "invariants", "testdata", "inv2")
	found := findAuditMutations(t, root, []string{"bad_audit.go"}, auditTables)
	if len(found) < 2 {
		t.Fatalf("detector INV-2 deveria acusar SQL cru e chamada ORM na fixture, acusou %d: %v", len(found), found)
	}
}

func TestSelfINV3DetectsForbiddenImport(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "test", "invariants", "testdata", "inv3", "domain")
	vs, err := findForbiddenDomainImports(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("detector INV-3 deveria acusar exatamente 1 importação proibida, acusou %d: %v", len(vs), vs)
	}
}

func TestSelfINV4FailsClosedOnForbiddenConditionedAndUnknown(t *testing.T) {
	csv := strings.Join([]string{
		"example.com/ok,https://x/LICENSE,MIT",
		"example.com/gpl,https://x/LICENSE,GPL-3.0",
		"example.com/mpl,https://x/LICENSE,MPL-2.0",
		"example.com/unk,https://x/LICENSE,Unknown",
		"example.com/odd,https://x/LICENSE,EUPL-1.2",
	}, "\n")
	vs := classifyCSV(csv, nil, nil, nil)
	if len(vs) != 4 {
		t.Fatalf("classificador INV-4 deveria acusar 4 de 5 (fail-closed), acusou %d: %v", len(vs), vs)
	}
	for _, needle := range []string{"PROIBIDA", "condicionada", "não determinável", "revisão obrigatória"} {
		if !strings.Contains(strings.Join(vs, "\n"), needle) {
			t.Errorf("classificação esperada ausente: %s", needle)
		}
	}
}
