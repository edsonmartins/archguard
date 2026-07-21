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

// INV-5 (análise estática, T-018) — cenário da spec "Query sem predicado de
// tenant": código novo que consulte tabela de domínio tenant-scoped sem
// predicado de tenant FALHA este teste e o build é rejeitado.
//
// O detector varre os fontes Go de `internal/` (o mundo pgx — o código novo;
// as tabelas legadas ficam no XORM e entram quando seu acesso migrar), extrai
// os literais de string que são queries sobre as tabelas guardadas e exige:
//
//   - SELECT/UPDATE/DELETE: cláusula WHERE com predicado de escopo —
//     `organization_id` (eixo do tenant) ou `identity_id` (eixo da identidade,
//     sancionado pelas policies de RLS 0013/0014);
//   - INSERT: a coluna de escopo presente na lista de colunas.
//
// Limitação declarada: o detector cobre queries em LITERAIS (o padrão da casa:
// `const q = ...` com SQL explícito). SQL montado por concatenação dinâmica
// não é coberto — e também não é aceito em revisão. As migrations (.sql
// embutidos) são superfície separada: backfills legítimos operam tabela
// inteira sob controle do migrator.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// tenantScopedPgxTables are the tenant-scoped tables of the NEW (pgx) world —
// keep in sync with TENANT_INVENTORY.md and with the RLS lock of
// TestINV5RLSStaysEnabled. Every future tenant-scoped pgx table MUST be added
// here when created.
var tenantScopedPgxTables = []string{"membership", "role_assignment", "auth_session"}

type queryViolation struct {
	File  string
	Line  int
	Table string
	Query string
}

var (
	sqlVerbRe   = regexp.MustCompile(`(?is)\b(select|insert|update|delete)\b`)
	scopePredRe = regexp.MustCompile(`(?is)\b(organization_id|identity_id)\b\s*(=|in\b|<>|!=)`)
	scopeColRe  = regexp.MustCompile(`(?is)\b(organization_id|identity_id)\b`)
)

// tableTouchRe matches a query position reference (FROM/JOIN/UPDATE/INSERT
// INTO) of the table.
func tableTouchRe(table string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)\b(from|join|update|insert\s+into)\s+` + table + `\b`)
}

// analyzeQueryLiteral reports the guarded tables the literal queries without a
// tenant-scope predicate.
func analyzeQueryLiteral(lit string) []string {
	if !sqlVerbRe.MatchString(lit) {
		return nil // not a query — error messages etc.
	}
	var bad []string
	lower := strings.ToLower(lit)
	for _, table := range tenantScopedPgxTables {
		touch := tableTouchRe(table).FindString(lower)
		if touch == "" {
			continue
		}
		if strings.HasPrefix(strings.Fields(touch)[0], "insert") ||
			regexp.MustCompile(`(?is)insert\s+into\s+`+table+`\b`).MatchString(lower) {
			// INSERT: the scope column must be among the inserted columns.
			if !scopeColRe.MatchString(lower) {
				bad = append(bad, table)
			}
			continue
		}
		// SELECT/UPDATE/DELETE: a WHERE with a scope predicate is mandatory.
		_, after, found := strings.Cut(lower, "where")
		if !found || !scopePredRe.MatchString(after) {
			bad = append(bad, table)
		}
	}
	return bad
}

// findUnscopedTenantQueries walks the Go sources under dir (skipping tests and
// testdata) and returns every query literal lacking a tenant-scope predicate.
func findUnscopedTenantQueries(t *testing.T, dir string) []queryViolation {
	t.Helper()
	var out []queryViolation
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "testdata" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text := lit.Value
			if unquoted, err := strconv.Unquote(text); err == nil {
				text = unquoted
			}
			for _, table := range analyzeQueryLiteral(text) {
				out = append(out, queryViolation{
					File:  path,
					Line:  fset.Position(lit.Pos()).Line,
					Table: table,
					Query: strings.Join(strings.Fields(text), " "),
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("varredura: %v", err)
	}
	return out
}

// TestINV5NoUnscopedTenantQueries is the build-breaking gate: every query in
// `internal/` touching a tenant-scoped table carries a tenant-scope predicate.
func TestINV5NoUnscopedTenantQueries(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal")
	violations := findUnscopedTenantQueries(t, dir)
	for _, v := range violations {
		t.Errorf("INV-5: query sem predicado de tenant sobre %q em %s:%d — %s",
			v.Table, v.File, v.Line, v.Query)
	}
	if len(violations) > 0 {
		t.Fatalf("%d query(ies) sem predicado de tenant — corrija o código, nunca o detector (CLAUDE.md §3)", len(violations))
	}
}

// TestSelfINV5DetectsInjectedViolations proves the detector actually catches
// what it must: the testdata fixture carries three unscoped queries (SELECT,
// UPDATE and INSERT) and one properly scoped, and the detector flags exactly
// the three.
func TestSelfINV5DetectsInjectedViolations(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "test", "invariants", "testdata", "inv5")
	violations := findUnscopedTenantQueries(t, dir)
	if len(violations) != 3 {
		t.Fatalf("detector INV-5 deveria acusar exatamente as 3 violações injetadas, acusou %d: %+v",
			len(violations), violations)
	}
	tables := map[string]bool{}
	for _, v := range violations {
		tables[v.Table] = true
	}
	for _, want := range tenantScopedPgxTables {
		if !tables[want] {
			t.Errorf("violação injetada sobre %q não detectada", want)
		}
	}
}

// TestSelfINV5IgnoresNonQueries guards against false positives: error messages
// mentioning table names and scoped queries must pass.
func TestSelfINV5IgnoresNonQueries(t *testing.T) {
	for _, ok := range []string{
		"postgres: membership não encontrado",
		"sessão de auth_session inválida",
		`SELECT id FROM membership WHERE organization_id = $1`,
		`SELECT id FROM auth_session WHERE id = $1 AND identity_id = $2`,
		`UPDATE role_assignment SET role_id = $1 WHERE membership_id = $2 AND organization_id = $3`,
		`INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,$4)`,
		`SELECT relrowsecurity FROM pg_class WHERE relname = $1`,
	} {
		if bad := analyzeQueryLiteral(ok); len(bad) != 0 {
			t.Errorf("falso positivo para %q: %v", ok, bad)
		}
	}
	for _, bad := range []string{
		`SELECT id FROM membership WHERE status = 'active'`,
		`UPDATE auth_session SET status = 'revoked'`,
		`DELETE FROM role_assignment WHERE role_id = $1`,
	} {
		if got := analyzeQueryLiteral(bad); len(got) == 0 {
			t.Errorf("falso negativo: %q deveria violar", bad)
		}
	}
}
