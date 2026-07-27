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

package object

import (
	"fmt"

	"github.com/xorm-io/core"
)

// controlPlaneApplication is the console's login application. A PAM control plane must
// never let anyone self-register into it — identities are provisioned (seed/admin/SCIM),
// never by self-signup (ADR-0021).
const controlPlaneApplication = "app-built-in"

// controlPlaneSignupIsOpen reports whether the control-plane application would let the
// public self-register. Pure decision (testable without a DB).
func controlPlaneSignupIsOpen(app *Application) bool {
	return app != nil && app.EnableSignUp
}

// EnforceNoSelfSignupOnControlPlane disables self-service registration on the control-plane
// application at boot. It is SELF-HEALING: if the flag drifted on — Casdoor's historical
// default, or an accidental toggle in the admin UI — it is forced off and a warning is
// logged, so the identity control plane can never be opened to the public (ADR-0021). Runs
// in every profile; a no-op when the app is already closed or not yet seeded.
func EnforceNoSelfSignupOnControlPlane() {
	app, err := getApplication("admin", controlPlaneApplication)
	if err != nil || !controlPlaneSignupIsOpen(app) {
		return
	}
	app.EnableSignUp = false
	affected, err := ormer.Engine.ID(core.PK{app.Owner, app.Name}).AllCols().Update(app)
	if err != nil {
		fmt.Printf("[ArchGuard] AVISO: falha ao desativar o auto-registro do plano de controle: %v\n", err)
		return
	}
	if affected > 0 {
		fmt.Printf("[ArchGuard] AVISO: auto-registro estava ATIVO no plano de controle (%s) e foi DESATIVADO (ADR-0021).\n", controlPlaneApplication)
	}
}
