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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBootUsesRealGlobalAuthorizer: o boot (internal/boot, exceto testes) NÃO pode usar os
// adapters PROVISIONAIS de acesso cross-tenant — ProfileAuthorizer (permite só em dev, nega
// em conforme) e MemoryAuditor (não durável). Eles ficam restritos a testes sem pool. Se o
// boot os reintroduzisse, a ponte de login voltaria a NEGAR em perfil conforme (production),
// derrubando toda a sessão do /api/v1 — foi o incidente que o ADR-0022 corrigiu ao ligar o
// ScopedAuthorizer + AccessAuditor durável (via newGlobalRepository). Este é o catch estático
// de regressão (I-1.3: o login não pode ficar fail-closed por ausência do authorizer real).
func TestBootUsesRealGlobalAuthorizer(t *testing.T) {
	bootDir := filepath.Join(repoRoot(t), "internal", "boot")
	entries, err := os.ReadDir(bootDir)
	if err != nil {
		t.Fatalf("lendo internal/boot: %v", err)
	}
	forbidden := []string{"NewProfileAuthorizer", "NewMemoryAuditor"}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(bootDir, name))
		if err != nil {
			t.Fatalf("lendo %s: %v", name, err)
		}
		checked++
		for _, bad := range forbidden {
			if strings.Contains(string(src), bad) {
				t.Errorf("internal/boot/%s usa o adapter provisional %q — o boot DEVE usar o "+
					"GlobalAuthorizer/auditor real (newGlobalRepository, ADR-0022); o provisional "+
					"negaria o login em perfil conforme (I-1.3)", name, bad)
			}
		}
	}
	if checked == 0 {
		t.Fatal("nenhum arquivo .go inspecionado em internal/boot — heurística quebrada")
	}
}
