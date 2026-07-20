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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// INV-4: no dependency outside the license matrix of ADR-0002 §3 in the
// build tree. The scan is FAIL-CLOSED (ADR-0002 §3a): a license that cannot
// be determined is a violation, not a warning. There is NO allowlist.
//
// Tool: go-licenses, pinned version, invoked via `go run pkg@version`
// (approved CI-tool class, ADR-0002 §3a — never in the main go.mod).
const goLicensesVersion = "v1.6.0"

var (
	allowedLicenses = map[string]bool{
		"Apache-2.0":   true,
		"MIT":          true,
		"BSD-2-Clause": true,
		"BSD-3-Clause": true,
		"ISC":          true,
		"Unlicense":    true,
		"Zlib":         true,
		"FTL":          true, // permissiva (estilo BSD c/ atribuição); elegível para eleição
	}
	forbiddenPrefixes = []string{
		"AGPL", "GPL", "LGPL", "SSPL", "BUSL", "Elastic",
	}
	// Conditioned classes (MPL/EPL/CDDL) are allowed ONLY as an external
	// service in a separate process (I-2.2); as a linked build dependency
	// they are a violation.
	conditionedPrefixes = []string{"MPL", "EPL", "CDDL"}
)

// classifyLicense returns an empty string when the license is allowed, or a
// violation description otherwise.
func classifyLicense(pkg, license string) string {
	license = strings.TrimSpace(license)
	if allowedLicenses[license] {
		return ""
	}
	for _, p := range forbiddenPrefixes {
		if strings.HasPrefix(license, p) {
			return fmt.Sprintf("INV-4: %s sob licença PROIBIDA %s (ADR-0002 §3)", pkg, license)
		}
	}
	for _, p := range conditionedPrefixes {
		if strings.HasPrefix(license, p) {
			return fmt.Sprintf("INV-4: %s sob licença condicionada %s LINKADA à árvore de build — permitida apenas como serviço em processo separado (I-2.2)", pkg, license)
		}
	}
	if license == "" || strings.EqualFold(license, "Unknown") {
		return fmt.Sprintf("INV-4: licença de %s não determinável — fail-closed (ADR-0002 §3a)", pkg)
	}
	return fmt.Sprintf("INV-4: %s sob licença %s fora da matriz — classe de revisão obrigatória, aprovação caso a caso exigida (ADR-0002 §3)", pkg, license)
}

// classifyCSV returns violations, tolerating findings whose module@version is
// quarantined in license-baseline.txt. The authoritative quarantine manager
// (with all five locks: exact match, stale-entry, shrink-only, resolution path,
// empty-to-close) is tools/licensegate, run by `make sbom`; this suite reads the
// same baseline file only to tolerate known-inherited findings, keeping INV-4 a
// self-contained build-breaking check for any NEW violation.
func classifyCSV(csv string, pkgMod, baseline, elections map[string]string) []string {
	var violations []string
	for _, line := range strings.Split(csv, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, ",")
		if len(cols) < 3 {
			violations = append(violations, fmt.Sprintf("INV-4: linha de relatório ilegível (fail-closed): %q", line))
			continue
		}
		pkg := cols[0]
		license := cols[len(cols)-1]
		// An explicit election (LICENSE_ELECTIONS.md) resolves an undeterminable
		// license to a permitted one — same authority licensegate applies.
		if elected := electionFor(pkg, elections); elected != "" && allowedLicenses[elected] {
			license = elected
		}
		if v := classifyLicense(pkg, license); v != "" {
			if _, quarantined := baseline[resolveModuleVer(pkg, pkgMod)]; quarantined {
				continue
			}
			violations = append(violations, v)
		}
	}
	return violations
}

// electionFor returns the elected license for a package, matching an election
// key that equals or prefixes the package path.
func electionFor(pkg string, elections map[string]string) string {
	if lic, ok := elections[pkg]; ok {
		return lic
	}
	for mod, lic := range elections {
		if strings.HasPrefix(pkg, mod+"/") {
			return lic
		}
	}
	return ""
}

// readElections parses module=>license lines from LICENSE_ELECTIONS.md.
func readElections(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	data, err := os.ReadFile(filepath.Join(root, "docs", "upstream", "LICENSE_ELECTIONS.md"))
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatalf("INV-4: LICENSE_ELECTIONS.md ilegível: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if !strings.Contains(line, "=>") {
			continue
		}
		parts := strings.SplitN(line, "=>", 2)
		mod := strings.TrimSpace(parts[0])
		lic := strings.TrimSpace(parts[1])
		if mod != "" && lic != "" {
			out[mod] = lic
		}
	}
	return out
}

// resolveModuleVer maps a go-licenses finding path to module@version, falling
// back to any build-graph package under the finding path.
func resolveModuleVer(findingPath string, pkgMod map[string]string) string {
	if mv, ok := pkgMod[findingPath]; ok {
		return mv
	}
	for pkg, mv := range pkgMod {
		if pkg == findingPath || strings.HasPrefix(pkg, findingPath+"/") {
			return mv
		}
	}
	return findingPath + "@?"
}

// baselineKeys reads the module@version keys quarantined in license-baseline.txt.
func baselineKeys(t *testing.T, root string) map[string]string {
	t.Helper()
	keys := map[string]string{}
	data, err := os.ReadFile(filepath.Join(root, "license-baseline.txt"))
	if os.IsNotExist(err) {
		return keys
	}
	if err != nil {
		t.Fatalf("INV-4: baseline ilegível: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "|"); i > 0 {
			keys[line[:i]] = line
		}
	}
	return keys
}

// buildModuleMap maps each build-graph package to its module@version.
func buildModuleMap(t *testing.T, root string) map[string]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("INV-4: go list falhou: %v", err)
	}
	m := map[string]string{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p struct {
			ImportPath string
			Module     *struct{ Path, Version string }
		}
		if err := dec.Decode(&p); err != nil {
			break
		}
		if p.Module != nil && p.Module.Path != "" {
			m[p.ImportPath] = p.Module.Path + "@" + p.Module.Version
		}
	}
	return m
}

func TestINV4LicenseMatrix(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "run", "github.com/google/go-licenses@"+goLicensesVersion, "csv", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("INV-4: scan de licenças falhou (fail-closed): %v\n%s", err, stderr)
	}
	pkgMod := buildModuleMap(t, root)
	baseline := baselineKeys(t, root)
	elections := readElections(t, root)
	for _, v := range classifyCSV(string(out), pkgMod, baseline, elections) {
		t.Error(v)
	}
}
