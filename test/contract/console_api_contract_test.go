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

// Package contract guarda o teste de contrato entre o console e o /api/v1 do plano
// de controle (pacote 008, T-002). É verificação estática (sem DB): garante que toda
// rota /api/v1 que o console chama existe montada no backend — detecta *drift* de
// contrato no CI (spec admin-console, cenário "Contrato defasado").
package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"
)

// repoRoot deriva a raiz do repositório a partir da localização deste arquivo
// (<root>/test/contract/console_api_contract_test.go), sem depender do CWD.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller falhou")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler %s: %v", path, err)
	}
	return string(b)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestConsoleControlPlaneContract: toda rota /api/v1 chamada pelo console
// (web/src/backend/ControlPlaneBackend.js, via cpRequest) DEVE estar montada no
// backend (internal/boot/mounts.go, via RegisterAPIHandler). O CI falha se o console
// passar a chamar uma rota que o backend não serve — I-7.6 (o endpoint público vem
// antes da tela) e a garantia anti-*drift* do pacote 008.
func TestConsoleControlPlaneContract(t *testing.T) {
	root := repoRoot(t)
	console := mustRead(t, filepath.Join(root, "web", "src", "backend", "ControlPlaneBackend.js"))
	mounts := mustRead(t, filepath.Join(root, "internal", "boot", "mounts.go"))

	mounted := map[string]bool{}
	for _, m := range regexp.MustCompile(`RegisterAPIHandler\("(/[^"]+)"`).FindAllStringSubmatch(mounts, -1) {
		mounted[m[1]] = true
	}
	if len(mounted) == 0 {
		t.Fatal("nenhuma rota RegisterAPIHandler encontrada em mounts.go — o padrão de registro mudou; atualize este teste")
	}

	// cpRequest("<MÉTODO>", "<caminho literal>", ...): captura só chamadas com caminho
	// literal (o helper genérico e caminhos parametrizados por template não entram).
	called := map[string]bool{}
	for _, m := range regexp.MustCompile(`cpRequest\("[A-Z]+",\s*"(/[^"]+)"`).FindAllStringSubmatch(console, -1) {
		called[m[1]] = true
	}
	if len(called) == 0 {
		t.Fatal("nenhuma chamada cpRequest com caminho literal encontrada em ControlPlaneBackend.js — o padrão mudou; atualize este teste")
	}

	for _, path := range keys(called) {
		if !mounted[path] {
			t.Errorf("DRIFT de contrato: o console chama %q mas o backend não monta essa rota /api/v1 "+
				"(RegisterAPIHandler em internal/boot/mounts.go). I-7.6: publique o endpoint antes da tela.", path)
		}
	}
}
