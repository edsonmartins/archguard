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

// Command lintbaseline runs `go vet -json ./...` and reconciles the findings
// against lint-baseline.txt (design.md 001, "Nota transitória"). Locks:
//
//	(a) exact match only — entries are file:line:check; no globs, no
//	    package-level suppression;
//	(b) a stale entry (no longer matching a real finding) breaks the build;
//	(c) the baseline only shrinks after package 001 closes — new code is
//	    born clean, no exceptions;
//	(d) any file touched by a future task leaves the baseline in that same
//	    commit (boy scout on touch, never on sight).
//
// Locks (c) and (d) are review rules made visible by (a)+(b): any addition or
// drift shows up as an explicit diff on this file and as a red build.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const baselineFile = "lint-baseline.txt"

var entryPattern = regexp.MustCompile(`^([^\s*?\[\]{}]+\.go):(\d+):([a-z0-9_]+)$`)

func loadBaseline(path string) (map[string]bool, error) {
	entries := map[string]bool{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !entryPattern.MatchString(line) {
			return nil, fmt.Errorf("%s:%d: entrada inválida %q — apenas \"arquivo.go:linha:check\"; glob e supressão por pacote são proibidos", baselineFile, i+1, line)
		}
		entries[line] = true
	}
	return entries, nil
}

// vetFindings runs go vet -json and returns findings as file:line:check with
// repo-relative paths.
func vetFindings(root string) ([]string, error) {
	cmd := exec.Command("go", "vet", "-json", "./...")
	cmd.Dir = root
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	// go vet -json exits non-zero on findings in some versions; the JSON on
	// stderr/stdout is still the source of truth.
	_ = cmd.Run()
	payload := out.String() + "\n" + stderr.String()

	var findings []string
	dec := json.NewDecoder(strings.NewReader(stripHashLines(payload)))
	for {
		var block map[string]map[string][]struct {
			Posn string `json:"posn"`
		}
		if err := dec.Decode(&block); err != nil {
			break
		}
		for _, checks := range block {
			for check, diags := range checks {
				for _, d := range diags {
					parts := strings.Split(d.Posn, ":")
					if len(parts) < 2 {
						continue
					}
					rel, err := filepath.Rel(root, parts[0])
					if err != nil {
						rel = parts[0]
					}
					findings = append(findings, fmt.Sprintf("%s:%s:%s", filepath.ToSlash(rel), parts[1], check))
				}
			}
		}
	}
	return findings, nil
}

func stripHashLines(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	baseline, err := loadBaseline(filepath.Join(root, baselineFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-baseline: %v\n", err)
		os.Exit(1)
	}
	findings, err := vetFindings(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-baseline: go vet: %v\n", err)
		os.Exit(1)
	}

	seen := map[string]bool{}
	failed := false
	for _, f := range findings {
		seen[f] = true
		if !baseline[f] {
			fmt.Fprintf(os.Stderr, "lint: achado fora do baseline: %s\n", f)
			failed = true
		}
	}
	for e := range baseline {
		if !seen[e] {
			fmt.Fprintf(os.Stderr, "lint: entrada OBSOLETA no baseline (trava b): %s — remova-a neste commit\n", e)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
	fmt.Printf("lint-baseline: ok (%d achados herdados tolerados, 0 novos)\n", len(baseline))
}
