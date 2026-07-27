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
	"regexp"
	"testing"
)

// TestControlPlaneSignupSeededDisabled: o seed da aplicação do plano de controle
// (object/init.go, initBuiltInApplication) NÃO pode habilitar auto-registro — um plano de
// controle de identidade provisiona identidades (seed/admin/SCIM) e a ponte de login é
// resolve-only; ninguém se auto-cadastra (ADR-0021). Este é o catch ESTÁTICO de regressão no
// seed; a barreira de runtime é object.EnforceNoSelfSignupOnControlPlane (self-healing no boot).
func TestControlPlaneSignupSeededDisabled(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "object", "init.go"))
	if err != nil {
		t.Fatalf("lendo object/init.go: %v", err)
	}
	if regexp.MustCompile(`EnableSignUp:\s*true`).Match(src) {
		t.Fatalf("object/init.go habilita auto-registro no seed (EnableSignUp: true) — proibido no plano de controle (ADR-0021)")
	}
}
