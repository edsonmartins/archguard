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

// I-3.3 (pétreo) / ADR-0014 §2 (pacote 010, T-017 / spec "Classificação
// obrigatória de dados pessoais"): toda migration que adiciona um CAMPO PESSOAL
// declara categoria, finalidade, base legal e prazo de retenção. Este é o gate
// automatizado que o 0006 registrou como follow-up: uma migration que adiciona
// uma coluna pessoal SEM a classificação LGPD QUEBRA O BUILD (make invariants).
//
// Heurística de "campo pessoal": coluna cujo nome termina em `_enc` (convenção de
// campo pessoal cifrado) ou contém `email`. A classificação vive no catálogo do
// banco: `COMMENT ON COLUMN <tabela>.<coluna> IS 'LGPD | categoria=… |
// finalidade=… | base_legal=… | retencao=…'`.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE TABLE(?:\s+IF NOT EXISTS)?\s+(\w+)\s*\((.*?)\)\s*;`)
	addColumnRe   = regexp.MustCompile(`(?is)ALTER TABLE\s+(\w+)(.*?);`)
	addColRe      = regexp.MustCompile(`(?is)ADD COLUMN(?:\s+IF NOT EXISTS)?\s+(\w+)`)
	commentRe     = regexp.MustCompile(`(?is)COMMENT ON COLUMN\s+(\w+)\.(\w+)\s+IS\s+'(.*?)'\s*;`)
)

// isPersonalColumn reports whether a column name is a personal field by convention.
func isPersonalColumn(col string) bool {
	c := strings.ToLower(col)
	return strings.HasSuffix(c, "_enc") || strings.Contains(c, "email")
}

// hasFullClassification reports whether an LGPD comment body carries all four
// required fields.
func hasFullClassification(body string) bool {
	b := strings.ToLower(body)
	return strings.Contains(b, "lgpd") &&
		strings.Contains(b, "categoria=") &&
		strings.Contains(b, "finalidade=") &&
		strings.Contains(b, "base_legal=") &&
		strings.Contains(b, "retencao=")
}

func constraintLine(s string) bool {
	switch strings.ToUpper(strings.Fields(s)[0]) {
	case "PRIMARY", "UNIQUE", "CHECK", "FOREIGN", "CONSTRAINT", "EXCLUDE":
		return true
	default:
		return false
	}
}

// TestINV3_3PersonalFieldsClassified rejects the build if any migration adds a
// personal column without a full LGPD classification.
func TestINV3_3PersonalFieldsClassified(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "migrate", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lendo migrations: %v", err)
	}

	personal := map[string]string{} // "table.col" -> file (needing classification)
	classified := map[string]bool{} // "table.col" with a full LGPD comment

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("lendo %s: %v", e.Name(), err)
		}
		sql := string(raw)

		// CREATE TABLE columns.
		for _, m := range createTableRe.FindAllStringSubmatch(sql, -1) {
			table := m[1]
			for _, part := range strings.Split(m[2], ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				fields := strings.Fields(part)
				if len(fields) == 0 || constraintLine(part) {
					continue
				}
				col := fields[0]
				if isPersonalColumn(col) {
					personal[table+"."+col] = e.Name()
				}
			}
		}
		// ALTER TABLE ... ADD COLUMN.
		for _, m := range addColumnRe.FindAllStringSubmatch(sql, -1) {
			table := m[1]
			for _, c := range addColRe.FindAllStringSubmatch(m[2], -1) {
				if isPersonalColumn(c[1]) {
					personal[table+"."+c[1]] = e.Name()
				}
			}
		}
		// LGPD classifications.
		for _, m := range commentRe.FindAllStringSubmatch(sql, -1) {
			if hasFullClassification(m[3]) {
				classified[m[1]+"."+m[2]] = true
			}
		}
	}

	if len(personal) == 0 {
		t.Fatal("gate LGPD não encontrou nenhuma coluna pessoal — heurística ou parsing quebrado")
	}

	var missing []string
	for tc, file := range personal {
		if !classified[tc] {
			missing = append(missing, tc+" (em "+file+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("BUILD REJEITADO (I-3.3): campo(s) pessoal(is) sem classificação LGPD completa "+
			"(categoria/finalidade/base_legal/retencao):\n  %s", strings.Join(missing, "\n  "))
	}
}
