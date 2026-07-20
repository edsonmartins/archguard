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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyPermitted(t *testing.T) {
	for _, lic := range []string{"Apache-2.0", "MIT", "BSD-3-Clause", "ISC"} {
		if v, _ := classify("example.com/ok", lic, nil); v != "" {
			t.Errorf("licença permitida %s classificada como violação: %s", lic, v)
		}
	}
}

func TestClassifyForbidden(t *testing.T) {
	for _, lic := range []string{"GPL-3.0", "AGPL-3.0", "LGPL-2.1", "SSPL-1.0", "BUSL-1.1"} {
		v, _ := classify("example.com/bad", lic, nil)
		if !strings.Contains(v, "PROIBIDA") {
			t.Errorf("licença proibida %s não acusada: %q", lic, v)
		}
	}
}

func TestClassifyMPLIsMPLAndRegimeAware(t *testing.T) {
	v, isMPL := classify("example.com/mpl", "MPL-2.0", nil)
	if !isMPL {
		t.Fatal("MPL-2.0 não marcada como MPL")
	}
	// mplLinkedAllowed=true (regime ADR-0019, ratificado 2026-07-20) ⇒ MPL
	// linkada não é achado; a modificação é decidida pelos detectores de
	// transição (checkMPLTransition), não por classify.
	if v != "" {
		t.Errorf("MPL linkada deveria ser permitida sob ADR-0019, veio: %q", v)
	}
}

// TestCollectFindingsAlwaysCollectsMPL guards the ADR-0019 safety mechanism:
// a permitted (unmodified) MPL module produces no finding, but MUST still be
// handed to the transition detectors. Regression test for the loop that once
// short-circuited on v=="" before recording the module as MPL, silently
// disarming checkMPLTransition for exactly the modules it needed to guard.
func TestCollectFindingsAlwaysCollectsMPL(t *testing.T) {
	comps := []component{
		{Ref: "github.com/hashicorp/go-uuid", License: "MPL-2.0"},
		{Ref: "example.com/ok", License: "MIT"},
		{Ref: "example.com/bad", License: "GPL-3.0"},
	}
	pkgMod := map[string]string{
		"github.com/hashicorp/go-uuid": "github.com/hashicorp/go-uuid@v1.0.3",
		"example.com/bad":              "example.com/bad@v1.0.0",
	}
	findings, mpl := collectFindings(comps, pkgMod, nil)

	if len(mpl) != 1 || mpl[0] != "github.com/hashicorp/go-uuid" {
		t.Fatalf("MPL permitida deveria ser coletada para os detectores de transição, veio: %v", mpl)
	}
	if _, ok := findings["github.com/hashicorp/go-uuid@v1.0.3"]; ok {
		t.Error("MPL permitida (ADR-0019) não deveria virar achado")
	}
	if _, ok := findings["example.com/bad@v1.0.0"]; !ok {
		t.Error("licença proibida deveria virar achado")
	}
}

func TestClassifyUnknownFailsClosed(t *testing.T) {
	for _, lic := range []string{"", "Unknown", "NOASSERTION"} {
		v, _ := classify("example.com/x", lic, nil)
		if !strings.Contains(v, "fail-closed") {
			t.Errorf("licença desconhecida %q deveria falhar fechado: %q", lic, v)
		}
	}
}

func TestClassifyDualRequiresElection(t *testing.T) {
	// Sem eleição registrada ⇒ desconhecida.
	v, _ := classify("github.com/golang/freetype", "FTL OR GPL-2.0", map[string]string{})
	if !strings.Contains(v, "sem eleição") {
		t.Errorf("dual sem eleição deveria falhar: %q", v)
	}
	// Com eleição permitida (FTL) ⇒ ok.
	v, _ = classify("github.com/golang/freetype", "FTL OR GPL-2.0", map[string]string{"github.com/golang/freetype": "FTL"})
	if v != "" {
		t.Errorf("dual com eleição FTL deveria passar: %q", v)
	}
	// Eleição para opção não ofertada ⇒ falha.
	v, _ = classify("github.com/golang/freetype", "FTL OR GPL-2.0", map[string]string{"github.com/golang/freetype": "MIT"})
	if !strings.Contains(v, "não consta") {
		t.Errorf("eleição inválida deveria falhar: %q", v)
	}
}

func TestClassifyUnknownResolvedByElection(t *testing.T) {
	// go-licenses reporta pacotes como Unknown; eleição por prefixo do módulo resolve.
	el := map[string]string{"github.com/golang/freetype": "FTL"}
	if v, _ := classify("github.com/golang/freetype/raster", "Unknown", el); v != "" {
		t.Errorf("unknown com eleição FTL (por prefixo) deveria passar: %q", v)
	}
	// Eleição para licença proibida não salva.
	el2 := map[string]string{"example.com/x": "GPL-3.0"}
	if v, _ := classify("example.com/x/pkg", "Unknown", el2); !strings.Contains(v, "PROIBIDA") {
		t.Errorf("eleição para GPL deveria falhar como proibida: %q", v)
	}
}

func TestParseReplacesBlockAndLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "go.mod")
	content := `module x
go 1.25
replace github.com/a/b => ../local/b
replace (
	github.com/c/d => github.com/fork/d v1.2.3
	github.com/e/f => ./vendored/f
)
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := parseReplaces(p)
	if err != nil {
		t.Fatal(err)
	}
	if !isLocalPath(r["github.com/a/b"]) {
		t.Errorf("a/b deveria ser local: %q", r["github.com/a/b"])
	}
	if isLocalPath(r["github.com/c/d"]) {
		t.Errorf("c/d NÃO é local (fork com versão): %q", r["github.com/c/d"])
	}
	if !isLocalPath(r["github.com/e/f"]) {
		t.Errorf("e/f deveria ser local: %q", r["github.com/e/f"])
	}
}

func TestMPLTransitionDetectsLocalReplace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module x\ngo 1.25\nreplace github.com/hashicorp/golang-lru => ../patched/golang-lru\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// go mod verify vai falhar (sem módulos), mas o detector (b) deve acusar o replace local antes.
	vs := checkMPLTransition(dir, []string{"github.com/hashicorp/golang-lru"})
	found := false
	for _, v := range vs {
		if strings.Contains(v, "replace") && strings.Contains(v, "golang-lru") {
			found = true
		}
	}
	if !found {
		t.Fatalf("detector (b) não acusou replace local de módulo MPL: %v", vs)
	}
}

func TestMPLTransitionDetectsVendoredMPL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\ngo 1.25\n"), 0o600)
	vp := filepath.Join(dir, "vendor", "github.com", "hashicorp", "go-uuid")
	if err := os.MkdirAll(vp, 0o755); err != nil {
		t.Fatal(err)
	}
	vs := checkVendorAltered(dir, []string{"github.com/hashicorp/go-uuid"})
	if len(vs) != 1 || !strings.Contains(vs[0], "vendorizado") {
		t.Fatalf("detector (c) não acusou módulo MPL vendorizado: %v", vs)
	}
}

func TestParseBaselineLocks(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "license-baseline.txt")
	content := strings.Join([]string{
		"# comentário",
		"github.com/hashicorp/golang-lru@v0.5.4|MPL-2.0|regime:ADR-0019", // ok
		"github.com/x/y@v1.0.0|Unknown|remocao:T-012",                    // ok
		"github.com/glob/*@v1|MIT|remocao:T-008",                         // trava a: glob
		"github.com/z/w@v2|MPL-2.0|aceito",                               // trava d: resolução inválida
	}, "\n")
	os.WriteFile(p, []byte(content), 0o600)
	entries, errs := parseBaseline(p)
	if len(entries) != 2 {
		t.Fatalf("esperado 2 entradas válidas, veio %d", len(entries))
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "trava a") {
		t.Errorf("glob deveria violar trava a: %v", errs)
	}
	if !strings.Contains(joined, "trava d") {
		t.Errorf("resolução inválida deveria violar trava d: %v", errs)
	}
	if e := entries["github.com/hashicorp/golang-lru@v0.5.4"]; e.resolution != "regime:ADR-0019" {
		t.Errorf("resolução não lida: %+v", e)
	}
}

func TestResolutionRegex(t *testing.T) {
	ok := []string{"remocao:T-008", "remocao:T-010a", "eleicao:LICENSE_ELECTIONS.md", "regime:ADR-0019"}
	bad := []string{"remocao:T008", "aceito", "remocao", "regime:ADR", "eleicao:outro.md"}
	for _, s := range ok {
		if !resolutionRe.MatchString(s) {
			t.Errorf("resolução válida rejeitada: %q", s)
		}
	}
	for _, s := range bad {
		if resolutionRe.MatchString(s) {
			t.Errorf("resolução inválida aceita: %q", s)
		}
	}
}

func TestParseElections(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "LICENSE_ELECTIONS.md")
	content := "# comentário\n```\ngithub.com/golang/freetype => FTL   # dual FTL-ou-GPLv2\n```\n"
	os.WriteFile(p, []byte(content), 0o600)
	e, err := parseElections(p)
	if err != nil {
		t.Fatal(err)
	}
	if e["github.com/golang/freetype"] != "FTL" {
		t.Fatalf("eleição não lida corretamente: %v", e)
	}
}
