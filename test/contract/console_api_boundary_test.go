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

package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRawControlPlaneAccessOutsideModule é a trava automatizável da T-003 (pacote
// 008): o `/api/v1` (plano de controle) só pode ser referenciado pela camada dedicada
// `web/src/backend/ControlPlaneBackend.js`. Nenhum outro arquivo do console pode fazer
// chamada crua ao `/api/v1` — todo acesso passa pela camada tipada (fail-closed,
// distinção denied×error, ponto único de contrato do T-002).
//
// As demais garantias da T-003 (nenhum endpoint "só para a UI" — I-7.6; nenhuma
// decisão de autorização no frontend — a API nega mesmo com o controle oculto) são de
// REVISÃO de PR; a garantia de backend correspondente é o gate de assurance/RequireAdmin
// e a spec admin-console (cenário "Elemento oculto").
func TestNoRawControlPlaneAccessOutsideModule(t *testing.T) {
	root := repoRoot(t)
	webSrc := filepath.Join(root, "web", "src")
	allowed := filepath.Join(webSrc, "backend", "ControlPlaneBackend.js")

	var offenders []string
	err := filepath.WalkDir(webSrc, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if path == allowed || !strings.HasSuffix(path, ".js") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), "/api/v1") {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("varrer web/src: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("acesso cru ao /api/v1 fora de ControlPlaneBackend.js em %d arquivo(s): %v — "+
			"todo acesso ao plano de controle passa pela camada ControlPlaneBackend (pacote 008, T-001/T-003)",
			len(offenders), offenders)
	}
}
