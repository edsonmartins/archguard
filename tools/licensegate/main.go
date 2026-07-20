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

// Command licensegate is the T-019a license gate. Given a CycloneDX SBOM and
// the repository root, it enforces the ADR-0002 matrix (fail-closed on unknown
// licenses), verifies the MPL-transition detectors of ADR-0019 §II.3, and
// checks that every dual-licensed dependency has a registered election.
//
// The MPL-transition detectors are DORMANT with respect to permission
// semantics: they run under both regimes (current = MPL linked forbidden;
// ADR-0019 ratified = MPL linked permitted when unmodified). Ratifying ADR-0019
// flips `mplLinkedAllowed`, not the detectors.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// mplLinkedAllowed reflects the VIGENTE license regime. ADR-0019 (ratified
// 2026-07-20) amended I-2.2: MPL/EPL/CDDL linked to the build tree is permitted
// WHEN NOT MODIFIED. The transition detectors (§II.3) still catch a modified MPL
// module — flipping this flag changed the permission, not the detectors.
const mplLinkedAllowed = true

var allowedLicenses = map[string]bool{
	"Apache-2.0": true, "MIT": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"ISC": true, "Unlicense": true, "Zlib": true, "FTL": true,
}

var (
	forbiddenPrefixes   = []string{"AGPL", "GPL", "LGPL", "SSPL", "BUSL", "Elastic"}
	conditionedPrefixes = []string{"MPL", "EPL", "CDDL"}
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: licensegate <repo-root> <sbom.json>")
		os.Exit(2)
	}
	root, sbomPath := os.Args[1], os.Args[2]

	var violations []string

	// O CycloneDX é o artefato de inventário; sua presença é exigida, mas a
	// CLASSIFICAÇÃO usa go-licenses (fonte autoritativa), cuja detecção é muito
	// superior à do cyclonedx-gomod. Fail-closed se o artefato não existir.
	if _, err := os.Stat(sbomPath); err != nil {
		fmt.Fprintf(os.Stderr, "license-gate: SBOM ausente em %s (fail-closed): %v\n", sbomPath, err)
		os.Exit(1)
	}

	elections, err := parseElections(filepath.Join(root, "docs", "upstream", "LICENSE_ELECTIONS.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "license-gate: eleições ilegíveis (fail-closed): %v\n", err)
		os.Exit(1)
	}

	baseline, baseErrs := parseBaseline(filepath.Join(root, "license-baseline.txt"))
	violations = append(violations, baseErrs...)

	comps, err := runGoLicenses(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "license-gate: go-licenses falhou (fail-closed): %v\n", err)
		os.Exit(1)
	}
	pkgMod, err := buildModuleMap(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "license-gate: `go list` falhou (fail-closed): %v\n", err)
		os.Exit(1)
	}

	// findings: problematic module@version -> normalized license.
	findings, mplModules := collectFindings(comps, pkgMod, elections)

	// Reconcile against the baseline (locks b and c).
	for modver, lic := range findings {
		want, ok := baseline[modver]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf("license-gate: achado NOVO fora do baseline: %s [%s] — resolva a licença ou remova o módulo (trava c)", modver, lic))
		case want.license != lic:
			violations = append(violations, fmt.Sprintf("license-gate: %s está no baseline como %s mas foi detectado como %s — atualize a entrada", modver, want.license, lic))
		}
	}
	for modver := range baseline {
		if _, ok := findings[modver]; !ok {
			violations = append(violations, fmt.Sprintf("license-gate: entrada OBSOLETA no baseline: %s — não corresponde a achado real; remova-a neste commit (trava b)", modver))
		}
	}

	violations = append(violations, checkMPLTransition(root, mplModules)...)

	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, v)
		}
		os.Exit(1)
	}
	fmt.Printf("license-gate: ok (%d pacotes; %d achados em quarentena no baseline; %d MPL não modificados)\n", len(comps), len(baseline), len(mplModules))
}

// collectFindings classifies each scanned component into (a) findings —
// module@version -> normalized license for anything not outright allowed — and
// (b) mplModules — EVERY MPL/EPL/CDDL package, allowed or not. A permitted MPL
// (classify returns v=="") is still collected into mplModules because the
// transition detectors (checkMPLTransition) are the whole basis for allowing it
// linked (ADR-0019 §II.3); dropping it here would silently disarm that guard.
func collectFindings(comps []component, pkgMod, elections map[string]string) (map[string]string, []string) {
	findings := map[string]string{}
	var mplModules []string
	for _, c := range comps {
		v, isMPL := classify(c.Ref, c.License, elections)
		if isMPL {
			mplModules = append(mplModules, c.Ref)
		}
		if v == "" {
			continue
		}
		modver := resolveModule(c.Ref, pkgMod)
		findings[modver] = normalizeLicense(c.License)
	}
	return findings, mplModules
}

// normalizeLicense collapses undeterminable license strings to "Unknown" so
// they compare cleanly against baseline entries.
func normalizeLicense(license string) string {
	license = strings.TrimSpace(license)
	if license == "" || strings.EqualFold(license, "NOASSERTION") {
		return "Unknown"
	}
	return license
}

// baselineEntry is a quarantined finding with its declared resolution path.
type baselineEntry struct {
	license    string
	resolution string
}

var (
	baselineLine = regexp.MustCompile(`^([^\s*?\[\]{}|]+@[^\s|]+)\|([^\s|]+)\|(\S+)$`)
	resolutionRe = regexp.MustCompile(`^(remocao:T-\d+[a-z]?|eleicao:LICENSE_ELECTIONS\.md|regime:ADR-\d+)$`)
)

// parseBaseline reads license-baseline.txt enforcing lock (a) exact format and
// lock (d) a valid resolution path. It returns the entries plus any format
// violations (never a silent skip).
func parseBaseline(path string) (map[string]baselineEntry, []string) {
	out := map[string]baselineEntry{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, []string{fmt.Sprintf("license-gate: baseline ilegível (fail-closed): %v", err)}
	}
	var errs []string
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := baselineLine.FindStringSubmatch(line)
		if m == nil {
			errs = append(errs, fmt.Sprintf("license-baseline.txt:%d: formato inválido %q — apenas `modulo@versao|SPDX|resolucao` (trava a)", i+1, line))
			continue
		}
		if !resolutionRe.MatchString(m[3]) {
			errs = append(errs, fmt.Sprintf("license-baseline.txt:%d: resolução inválida %q — use remocao:T-0XX | eleicao:LICENSE_ELECTIONS.md | regime:ADR-0019 (trava d)", i+1, m[3]))
			continue
		}
		out[m[1]] = baselineEntry{license: m[2], resolution: m[3]}
	}
	return out, errs
}

// resolveModule maps a go-licenses finding path to its module@version. The
// finding path is a license root, often the module root, while go list reports
// the imported subpackages; so fall back to any build-graph package under the
// finding path.
func resolveModule(findingPath string, pkgMod map[string]string) string {
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

// buildModuleMap maps each package in the build graph to its module@version via
// `go list -deps -json ./...`.
func buildModuleMap(root string) (map[string]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p struct {
			ImportPath string `json:"ImportPath"`
			Module     *struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Module"`
		}
		if err := dec.Decode(&p); err != nil {
			break
		}
		if p.Module != nil && p.Module.Path != "" {
			m[p.ImportPath] = p.Module.Path + "@" + p.Module.Version
		}
	}
	return m, nil
}

// goLicensesVersion pins the authoritative license scanner (approved CI tool,
// ADR-0002 §3a). Kept in lockstep with the INV-4 invariant test.
const goLicensesVersion = "v1.6.0"

// runGoLicenses runs go-licenses over the main module and returns one component
// per package with its detected license. A line it cannot parse becomes an
// empty license (=> fail-closed downstream), never a silent skip.
func runGoLicenses(root string) ([]component, error) {
	cmd := exec.Command("go", "run", "github.com/google/go-licenses@"+goLicensesVersion, "csv", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return nil, fmt.Errorf("%v\n%s", err, stderr)
	}
	var comps []component
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, ",")
		lic := ""
		if len(cols) >= 3 {
			lic = strings.TrimSpace(cols[len(cols)-1])
		}
		comps = append(comps, component{Ref: cols[0], License: lic})
	}
	return comps, nil
}

// component is a package with its detected license.
type component struct {
	Ref     string
	License string
}

// classify applies the ADR-0002 matrix. Returns (violation, isMPL). A dual
// license (SPDX "A OR B") requires a registered election; otherwise it is
// treated as unknown => fail-closed.
func classify(module, license string, elections map[string]string) (string, bool) {
	// Dual license (SPDX "A OR B"): requires an election that names one of the
	// offered options.
	if strings.Contains(license, " OR ") {
		elected, ok := electionFor(module, elections)
		if !ok {
			return fmt.Sprintf("license-gate: %s é dual (%s) sem eleição registrada — desconhecida, fail-closed (ADR-0002 §3a; ver LICENSE_ELECTIONS.md)", module, license), false
		}
		if !strings.Contains(license, elected) {
			return fmt.Sprintf("license-gate: %s: licença eleita %q não consta das opções ofertadas (%s)", module, elected, license), false
		}
		license = elected
	}
	// Undeterminable license: a human may resolve it by explicit election (e.g.
	// dual-licensed deps that go-licenses reports as Unknown). The elected
	// license must itself be permitted; an unelected unknown is fail-closed.
	if license == "" || strings.EqualFold(license, "Unknown") || strings.EqualFold(license, "NOASSERTION") {
		if elected, ok := electionFor(module, elections); ok {
			license = elected
		}
	}
	if allowedLicenses[license] {
		return "", false
	}
	for _, p := range forbiddenPrefixes {
		if strings.HasPrefix(license, p) {
			return fmt.Sprintf("license-gate: %s sob licença PROIBIDA %s (ADR-0002 §3)", module, license), false
		}
	}
	for _, p := range conditionedPrefixes {
		if strings.HasPrefix(license, p) {
			if mplLinkedAllowed {
				return "", true // permitido linkado (ADR-0019) — sujeito aos detectores de transição
			}
			return fmt.Sprintf("license-gate: %s sob %s LINKADA — proibida no regime vigente (I-2.2; permitida só após ADR-0019)", module, license), true
		}
	}
	if license == "" || strings.EqualFold(license, "Unknown") || strings.EqualFold(license, "NOASSERTION") {
		return fmt.Sprintf("license-gate: licença de %s não determinável — fail-closed (ADR-0002 §3a; eleição possível em LICENSE_ELECTIONS.md)", module), false
	}
	return fmt.Sprintf("license-gate: %s sob licença %s fora da matriz — revisão obrigatória (ADR-0002 §3)", module, license), false
}

// electionFor returns the elected license for a package, matching an election
// key that equals or prefixes the package path (go-licenses reports package
// paths, elections are keyed by module path).
func electionFor(pkg string, elections map[string]string) (string, bool) {
	if lic, ok := elections[pkg]; ok {
		return lic, true
	}
	for mod, lic := range elections {
		if strings.HasPrefix(pkg, mod+"/") {
			return lic, true
		}
	}
	return "", false
}

// parseElections reads LICENSE_ELECTIONS.md, extracting lines of the form
// `<module> => <SPDX>` from fenced code blocks. Comments after `#` are ignored.
func parseElections(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
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
	return out, nil
}

// checkMPLTransition runs the three ADR-0019 §II.3 detectors on the main module
// at root. Any hit is a violation regardless of the permission regime.
func checkMPLTransition(root string, mplModules []string) []string {
	var violations []string

	replaces, err := parseReplaces(filepath.Join(root, "go.mod"))
	if err != nil {
		return []string{fmt.Sprintf("license-gate: go.mod ilegível (fail-closed): %v", err)}
	}
	// (b) no `replace` to a local path for any MPL module. go-licenses reports
	// package paths, so match a replace key that equals or prefixes the package.
	for _, m := range mplModules {
		for old, target := range replaces {
			if (m == old || strings.HasPrefix(m, old+"/")) && isLocalPath(target) {
				violations = append(violations, fmt.Sprintf("license-gate/transição: módulo MPL %s tem `replace` para caminho local %q — transição para MODIFICADO (ADR-0019 §II.3b)", old, target))
			}
		}
	}
	// (c) no altered vendoring of MPL files.
	if v := checkVendorAltered(root, mplModules); len(v) > 0 {
		violations = append(violations, v...)
	}
	// (a) hash vs proxy: `go mod verify` confirms the module cache matches the
	// proxy-verified go.sum. A modified MPL module in cache fails here.
	cmd := exec.Command("go", "mod", "verify")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		violations = append(violations, fmt.Sprintf("license-gate/transição: `go mod verify` falhou — integridade de módulo não confirmada vs proxy (ADR-0019 §II.3a): %s", strings.TrimSpace(string(out))))
	}
	return violations
}

// parseReplaces extracts `replace old => new` directives (both single-line and
// block form), returning old-module -> new-target.
func parseReplaces(goModPath string) (map[string]string, error) {
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	inBlock := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case strings.HasPrefix(line, "replace ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "replace "):
			addReplace(out, strings.TrimPrefix(line, "replace "))
		case inBlock && line != "":
			addReplace(out, line)
		}
	}
	return out, nil
}

func addReplace(out map[string]string, spec string) {
	parts := strings.SplitN(spec, "=>", 2)
	if len(parts) != 2 {
		return
	}
	oldMod := strings.Fields(strings.TrimSpace(parts[0]))
	newMod := strings.Fields(strings.TrimSpace(parts[1]))
	if len(oldMod) == 0 || len(newMod) == 0 {
		return
	}
	out[oldMod[0]] = newMod[0]
}

// isLocalPath reports whether a replace target is a filesystem path (as opposed
// to another module path with a version).
func isLocalPath(target string) bool {
	return strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") ||
		strings.HasPrefix(target, "/") || strings.HasPrefix(target, ".\\")
}

// checkVendorAltered flags any vendored MPL module tree: if the build vendors
// an MPL module, its files could diverge from the proxy without `go mod verify`
// noticing. Vendoring an MPL module is therefore treated as a transition risk
// that requires manual review.
func checkVendorAltered(root string, mplModules []string) []string {
	var violations []string
	for _, m := range mplModules {
		p := filepath.Join(root, "vendor", filepath.FromSlash(m))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			violations = append(violations, fmt.Sprintf("license-gate/transição: módulo MPL %s está vendorizado em vendor/ — árvore pode divergir do proxy sem detecção; revisão manual exigida (ADR-0019 §II.3c)", m))
		}
	}
	return violations
}
