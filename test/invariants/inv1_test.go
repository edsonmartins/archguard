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

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// INV-1: no code path authenticates a user with a credential that is not
// their own. The upstream "master password" mechanism (an organization-level
// credential accepted in place of the user's password) must not exist.
//
// Transitional allowlist (design.md 001, "Nota transitória"): inherited
// violations are tolerated ONLY when listed in known_violations.txt with an
// exact file:symbol entry. The file is deleted by T-011; after that, its very
// existence fails this suite. No other invariant accepts an allowlist.

const inv1Symbol = "MasterPassword"

var (
	inv1Pattern = regexp.MustCompile(`(?i)\bmaster_?password\b`)
	// MySQL "CHANGE MASTER TO ... master_password=..." is replication
	// vocabulary, not an authentication credential path (the code that uses
	// it is removed by T-012). Lines with the replication statement are the
	// single tolerated textual context.
	inv1ReplicationLine = regexp.MustCompile(`(?i)change\s+master\s+to`)
)

// allowlistEntry is an exact file:symbol pair. Globs are forbidden.
type allowlistEntry struct {
	File   string
	Symbol string
}

var allowlistLine = regexp.MustCompile(`^INV-1 ([^\s*?\[\]{}]+\.go):([A-Za-z0-9_]+)$`)

// parseAllowlist enforces the three structural locks:
// (a) exact match only — any glob/regex/directory pattern is a parse error;
// (b) staleness is checked by the caller against real findings;
// (c) an existing file that tolerates nothing (zero entries) is itself an
// error, so the file cannot outlive T-011.
func parseAllowlist(path string) ([]allowlistEntry, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []allowlistEntry
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := allowlistLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("known_violations.txt:%d: entrada inválida %q — apenas o formato exato \"INV-1 arquivo.go:Símbolo\" é aceito; glob, regex e diretório são proibidos", i+1, line)
		}
		entries = append(entries, allowlistEntry{File: m[1], Symbol: m[2]})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s existe mas não tolera nenhuma violação: o arquivo deveria ter sido deletado (subtarefa de T-011) — sua existência é violação de INV-1", filepath.Base(path))
	}
	return entries, nil
}

// findMasterCredential returns the repo-relative files matching the
// master-password pattern.
func findMasterCredential(t *testing.T, root string, files []string) []string {
	t.Helper()
	var found []string
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if inv1Pattern.MatchString(line) && !inv1ReplicationLine.MatchString(line) {
				found = append(found, rel)
				break
			}
		}
	}
	return found
}

// checkINV1 reconciles findings with the allowlist and returns violation
// messages. Exported logic kept test-internal on purpose: the suite is not a
// library.
func checkINV1(found []string, entries []allowlistEntry) []string {
	listed := map[string]bool{}
	for _, e := range entries {
		listed[e.File] = true
	}
	seen := map[string]bool{}
	for _, f := range found {
		seen[f] = true
	}
	var violations []string
	for _, f := range found {
		if !listed[f] {
			violations = append(violations, fmt.Sprintf("INV-1: caminho de credencial mestre em %s (símbolo %s) fora da allowlist transitória", f, inv1Symbol))
		}
	}
	for _, e := range entries {
		if !seen[e.File] {
			violations = append(violations, fmt.Sprintf("INV-1: entrada obsoleta na allowlist — %s:%s não corresponde mais a violação real; remova a entrada (ou delete o arquivo, se vazio — T-011)", e.File, e.Symbol))
		}
	}
	return violations
}

func TestINV1NoMasterCredential(t *testing.T) {
	root := repoRoot(t)
	entries, err := parseAllowlist(filepath.Join(root, "test", "invariants", "known_violations.txt"))
	if err != nil {
		t.Fatalf("INV-1: %v", err)
	}
	found := findMasterCredential(t, root, goSourceFiles(t, root))
	for _, v := range checkINV1(found, entries) {
		t.Error(v)
	}
}
