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

package domain

import "testing"

// O catálogo cobre todos os caminhos críticos que o design 010 lista, sem
// duplicatas, e cada SLI tem descrição.
func TestSLICatalogComplete(t *testing.T) {
	want := []CriticalPath{
		PathOIDCAuthz, PathTokenIssue, PathTokenRenew, PathMFAValidate,
		PathAuditWrite, PathPDPDecision, PathVaultCall,
	}
	cat := SLICatalog()
	if len(cat) != len(want) {
		t.Fatalf("esperava %d SLIs, veio %d", len(want), len(cat))
	}
	seen := map[CriticalPath]bool{}
	for _, s := range cat {
		if s.Description == "" {
			t.Fatalf("SLI %q sem descrição", s.Path)
		}
		if seen[s.Path] {
			t.Fatalf("SLI duplicado: %q", s.Path)
		}
		seen[s.Path] = true
	}
	for _, p := range want {
		if !seen[p] {
			t.Fatalf("caminho crítico %q ausente do catálogo", p)
		}
	}
}

// Só os objetivos COMPROMETIDOS pelo RFC-0001 §8 têm número; o resto é 0 (a
// definir no M2 — não inventado).
func TestSLIObjectivesFromRFC(t *testing.T) {
	tok, _ := SLIForPath(PathTokenIssue)
	if tok.LatencyObjectiveMillis != 150 {
		t.Fatalf("emissão de token deveria ter objetivo p95 150ms, veio %d", tok.LatencyObjectiveMillis)
	}
	pdp, _ := SLIForPath(PathPDPDecision)
	if pdp.LatencyObjectiveMillis != 50 {
		t.Fatalf("decisão do PDP deveria ter objetivo p95 50ms, veio %d", pdp.LatencyObjectiveMillis)
	}
	// Caminhos sem objetivo comprometido não têm número inventado.
	for _, p := range []CriticalPath{PathOIDCAuthz, PathTokenRenew, PathMFAValidate, PathAuditWrite, PathVaultCall} {
		s, _ := SLIForPath(p)
		if s.LatencyObjectiveMillis != 0 {
			t.Fatalf("caminho %q não tem objetivo comprometido no RFC-0001 — não deveria ter número (%d)", p, s.LatencyObjectiveMillis)
		}
	}
	if AuthPlaneAvailabilityObjective != 0.999 {
		t.Fatalf("disponibilidade do plano de auth deveria ser 99,9%%")
	}
}

func TestSLIForPathUnknown(t *testing.T) {
	if _, ok := SLIForPath("inexistente"); ok {
		t.Fatalf("caminho desconhecido não deveria existir")
	}
}
