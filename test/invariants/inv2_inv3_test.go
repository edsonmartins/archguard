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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// INV-2: no UPDATE/DELETE against audit tables, in any layer. The audit
// table set below grows with package 003 (audit_event etc.); "record" is the
// inherited upstream audit-ish table. There is NO allowlist for INV-2.
var auditTables = []string{"record"}

// auditMutationPatterns builds the per-table detection patterns: raw SQL
// (UPDATE <t> / DELETE FROM <t>) and ORM calls (.Update(&Record{...}) /
// .Delete(&Record{...})) — the struct name is the CamelCase of the table.
func auditMutationPatterns(tables []string) []*regexp.Regexp {
	var ps []*regexp.Regexp
	for _, table := range tables {
		structName := strings.ToUpper(table[:1]) + table[1:]
		ps = append(ps,
			regexp.MustCompile(`(?i)\bupdate\s+["'`+"`"+`]?`+table+`\b`),
			regexp.MustCompile(`(?i)\bdelete\s+from\s+["'`+"`"+`]?`+table+`\b`),
			regexp.MustCompile(`\.(Update|Delete)\(\s*&?(\w+\.)?`+structName+`\{`),
		)
	}
	return ps
}

func findAuditMutations(t *testing.T, root string, files []string, tables []string) []string {
	t.Helper()
	patterns := auditMutationPatterns(tables)
	var found []string
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		// Line-based scan: SQL statements and ORM calls fit in a line; a
		// whole-file match would let \s+ cross newlines and flag prose in
		// comments followed by unrelated identifiers.
		for n, line := range strings.Split(string(data), "\n") {
			for _, p := range patterns {
				if p.MatchString(line) {
					found = append(found, fmt.Sprintf("INV-2: mutação de tabela de auditoria em %s:%d (padrão %s)", rel, n+1, p))
				}
			}
		}
	}
	return found
}

func TestINV2AuditIsAppendOnly(t *testing.T) {
	root := repoRoot(t)
	for _, v := range findAuditMutations(t, root, goSourceFiles(t, root), auditTables) {
		t.Error(v)
	}
}

// INV-3: packages under internal/domain must not import the web framework or
// the ORM (ADR-0016). Passes vacuously while internal/domain does not exist
// (it is created by T-015) and activates automatically afterwards.
var forbiddenDomainImports = []string{
	"github.com/beego/beego",
	"github.com/xorm-io/",
	"github.com/casdoor/xorm-adapter",
}

func findForbiddenDomainImports(domainDir string) ([]string, error) {
	if _, err := os.Stat(domainDir); os.IsNotExist(err) {
		return nil, nil
	}
	var violations []string
	err := filepath.WalkDir(domainDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return err
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			ipath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenDomainImports {
				if strings.HasPrefix(ipath, forbidden) {
					violations = append(violations, fmt.Sprintf("INV-3: %s importa %s — pacote de domínio não importa framework web nem ORM (ADR-0016)", path, ipath))
				}
			}
		}
		return nil
	})
	return violations, err
}

func TestINV3DomainIsFrameworkFree(t *testing.T) {
	root := repoRoot(t)
	violations, err := findForbiddenDomainImports(filepath.Join(root, "internal", "domain"))
	if err != nil {
		t.Fatalf("INV-3: %v", err)
	}
	for _, v := range violations {
		t.Error(v)
	}
}
