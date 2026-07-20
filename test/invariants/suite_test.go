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

// Package invariants is the build-breaking invariant suite (INV-1..INV-8).
//
// It guards the fork against reintroduction of forbidden behavior via
// cherry-picks from upstream (ADR-0003). Never weaken a detector to make the
// build pass: fixing the code is the only allowed path (CLAUDE.md §3);
// changing an invariant test requires an ADR amendment.
package invariants

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks upward from the working directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root (go.mod) not found")
		}
		dir = parent
	}
}

// goSourceFiles returns repo-relative paths of .go files under root, skipping
// directories that are not part of the Go build surface under scrutiny:
// vendor trees, the web frontend, testdata fixtures and this suite itself.
func goSourceFiles(t *testing.T, root string) []string {
	t.Helper()
	skipDirs := map[string]bool{
		"vendor":       true,
		"web":          true,
		"testdata":     true,
		".git":         true,
		"node_modules": true,
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || rel == filepath.Join("test", "invariants") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}
