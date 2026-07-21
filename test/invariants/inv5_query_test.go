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

// tableMatcher holds the per-table regexps, compiled ONCE (the detector runs
// over every string literal under internal/, so recompiling per literal would
// be pure CI waste).
type tableMatcher struct {
	table  string
	touch  *regexp.Regexp // any read/write position: FROM/JOIN/UPDATE/DELETE FROM/INSERT INTO
	insert *regexp.Regexp // INSERT INTO <table> ( <column list> ) — group 1 is the columns
}

var tableMatchers = func() []tableMatcher {
	ms := make([]tableMatcher, 0, len(tenantScopedPgxTables))
	for _, t := range tenantScopedPgxTables {
		ms = append(ms, tableMatcher{
			table:  t,
			touch:  regexp.MustCompile(`(?is)\b(from|join|update|insert\s+into)\s+` + t + `\b`),
			insert: regexp.MustCompile(`(?is)\binsert\s+into\s+` + t + `\s*\(([^)]*)\)`),
		})
	}
	return ms
}()

// analyzeQueryLiteral reports the guarded tables the literal queries without a
// tenant-scope predicate.
func analyzeQueryLiteral(lit string) []string {
	if !sqlVerbRe.MatchString(lit) {
		return nil // not a query — error messages etc.
	}
	var bad []string
	for _, m := range tableMatchers {
		// INSERT: the scope column must be among the INSERTED columns — check the
		// column list captured between the parens, NOT the whole literal (a
		// column of a joined/selected table appearing elsewhere must not count).
		if cols := m.insert.FindStringSubmatch(lit); cols != nil {
			if !scopeColRe.MatchString(cols[1]) {
				bad = append(bad, m.table)
			}
			continue
		}
		if m.touch.FindString(lit) == "" {
			continue
		}
		// SELECT/UPDATE/DELETE (or an INSERT without an explicit column list): a
		// WHERE with a scope predicate is mandatory.
		_, after, found := strings.Cut(strings.ToLower(lit), "where")
		if !found || !scopePredRe.MatchString(after) {
			bad = append(bad, m.table)
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
		// INSERT ... SELECT com a coluna de escopo NA LISTA de colunas inseridas
		// (a fonte é `role`, não guardada) passa.
		`INSERT INTO role_assignment (id, organization_id, role_id, membership_id) SELECT gen_random_uuid(), $1, r.id, $2 FROM role r WHERE r.id = $3`,
	} {
		if bad := analyzeQueryLiteral(ok); len(bad) != 0 {
			t.Errorf("falso positivo para %q: %v", ok, bad)
		}
	}
	for _, bad := range []string{
		`SELECT id FROM membership WHERE status = 'active'`,
		`UPDATE auth_session SET status = 'revoked'`,
		`DELETE FROM role_assignment WHERE role_id = $1`,
		// A regressão do achado: INSERT cuja LISTA de colunas omite o escopo, mas
		// que menciona organization_id só no WHERE de uma tabela DIFERENTE. O
		// detector antigo (varria o literal inteiro) deixava passar.
		`INSERT INTO auth_session (id, membership_id) SELECT id, membership_id FROM membership m WHERE m.organization_id = $1 AND m.identity_id = $2`,
	} {
		if got := analyzeQueryLiteral(bad); len(got) == 0 {
			t.Errorf("falso negativo: %q deveria violar", bad)
		}
	}
}
