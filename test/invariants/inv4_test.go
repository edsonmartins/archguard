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
	"os/exec"
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

func classifyCSV(csv string) []string {
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
		if v := classifyLicense(cols[0], cols[len(cols)-1]); v != "" {
			violations = append(violations, v)
		}
	}
	return violations
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
	for _, v := range classifyCSV(string(out)) {
		t.Error(v)
	}
}
