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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// commit is one upstream commit under triage.
type commit struct {
	sha     string
	subject string
	files   []string
	class   Class
	reason  string
}

const upstreamRef = "vendor/upstream"

var shaLine = regexp.MustCompile(`([0-9a-f]{40})`)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	lastSHA, err := readLastSync(root + "/docs/upstream/LAST_SYNC.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "upstream-triage: %v\n", err)
		os.Exit(1)
	}

	if changed, err := licenseChanged(root, lastSHA); err != nil {
		fmt.Fprintf(os.Stderr, "upstream-triage: verificação de LICENSE falhou: %v\n", err)
	} else if changed {
		fmt.Println("‼️  INCIDENTE DE GOVERNANÇA: o arquivo LICENSE do upstream MUDOU desde o último sync.")
		fmt.Println("    Triagem em 48h (ADR-0002 §6). NÃO importe nada até a decisão.")
		fmt.Println()
	}

	commits, err := listCommits(root, lastSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upstream-triage: %v\n", err)
		os.Exit(1)
	}
	if len(commits) == 0 {
		fmt.Printf("upstream-triage: nenhum commit novo desde %s. Fila vazia.\n", short(lastSHA))
		return
	}

	report(lastSHA, commits)
}

// readLastSync extracts the last synced 40-hex SHA from LAST_SYNC.md.
func readLastSync(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("LAST_SYNC.md ilegível: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(strings.ToLower(line), "sha") {
			if m := shaLine.FindString(line); m != "" {
				return m, nil
			}
		}
	}
	return "", fmt.Errorf("nenhum SHA de 40 hex encontrado em LAST_SYNC.md")
}

// licenseChanged reports whether the upstream LICENSE differs between the last
// synced commit and the current upstream tip (ADR-0002 §6).
func licenseChanged(root, lastSHA string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet", lastSHA, upstreamRef, "--", "LICENSE")
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		return false, nil // exit 0 = no diff
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return true, nil // exit 1 = differs
	}
	return false, err
}

// listCommits returns commits on upstreamRef after lastSHA, newest first, with
// touched files and their classification.
func listCommits(root, lastSHA string) ([]commit, error) {
	// %x1f = unit separator between sha/subject; %x1e = record separator.
	out, err := runGit(root, "log", "--no-merges", "--name-only",
		"--format=%x1e%H%x1f%s", lastSHA+".."+upstreamRef)
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		head, rest, _ := strings.Cut(rec, "\n")
		sha, subject, _ := strings.Cut(head, "\x1f")
		var files []string
		for _, f := range strings.Split(rest, "\n") {
			if f = strings.TrimSpace(f); f != "" {
				files = append(files, f)
			}
		}
		c := commit{sha: sha, subject: subject, files: files}
		c.class, c.reason = Classify(subject, files)
		commits = append(commits, c)
	}
	return commits, nil
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr)
	}
	return string(out), nil
}

// order is the reporting priority: most urgent / actionable first.
var order = []Class{Security, DivergentReview, BugfixCandidate, FeaturePAM, OutOfScope, Refactor}

func report(lastSHA string, commits []commit) {
	byClass := map[Class][]commit{}
	for _, c := range commits {
		byClass[c.class] = append(byClass[c.class], c)
	}
	fmt.Printf("Fila de triagem de upstream — %d commits novos desde %s (%s)\n\n",
		len(commits), short(lastSHA), upstreamRef)

	for _, class := range order {
		cs := byClass[class]
		if len(cs) == 0 {
			continue
		}
		fmt.Printf("## %s (%d) — %s\n", class, len(cs), cs[0].reason)
		sort.Slice(cs, func(i, j int) bool { return cs[i].subject < cs[j].subject })
		for _, c := range cs {
			fmt.Printf("  %s  %s\n", short(c.sha), c.subject)
		}
		fmt.Println()
	}
	fmt.Println("Aja: SECURITY em 72h (cherry-pick c/ trailer Upstream-Commit ou mitigação);")
	fmt.Println("DIVERGENT-REVIEW manual; BUGFIX-CANDIDATE avaliar; FEATURE exige ADR/RFC;")
	fmt.Println("OUT-OF-SCOPE/REFACTOR descartar. Atualize LAST_SYNC.md ao concluir a rodada.")
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
