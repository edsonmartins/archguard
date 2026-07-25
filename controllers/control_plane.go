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

package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/casdoor/casdoor/internal/boot"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
	"github.com/google/uuid"
)

// HandleControlPlane is the Beego→net/http bridge for the ArchGuard control-plane
// API (pacote 011). It follows the proven pattern of HandleScim: a wildcard route
// (`/api/v1/*`) reaches this method, which applies the baseline gate, strips the
// version prefix and delegates to the composition root's mux (boot.APIHandler).
//
// Gate here is coarse authentication only — an established session. Per-operation
// authorization (tenant scoping and assurance level L1/L2/L3) is applied by the
// mounted handlers themselves (T-004+), not here. Both a missing session and a
// not-yet-mounted API fail closed (401 / 503), never open access.
//
// The version probe (/api/v1/version) is the ONE public exception: it is a
// liveness/version check (health checks, uptime monitors, the generated client's
// compatibility assertion) that carries no data and needs no session.
func (c *RootController) HandleControlPlane() {
	isPublicProbe := c.Ctx.Request.URL.Path == boot.APIBasePath+"/version"
	if !isPublicProbe && c.GetSessionUsername() == "" {
		c.Ctx.ResponseWriter.WriteHeader(http.StatusUnauthorized)
		return
	}

	handler := boot.APIHandler()
	if handler == nil {
		// The composition root has not mounted the API — refuse, do not fall open.
		c.Ctx.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	req := c.Ctx.Request
	// Bind the new-model identity+session that the login bridge (T-004b) wrote into
	// the framework session, so the domain pipeline can resolve the auth_session. A
	// missing or malformed binding is simply not injected — the pipeline then treats
	// the request as unauthenticated (fail-closed), never open.
	if idv, sidv := c.GetSession(boot.SessionKeyIdentityID), c.GetSession(boot.SessionKeyAuthSession); idv != nil && sidv != nil {
		if id, ok := parseSessionUUID(idv); ok {
			if sid, ok := parseSessionUUID(sidv); ok {
				req = req.WithContext(boot.WithSessionBinding(req.Context(), id, sid))
			}
		}
	}
	// Carry the caller's legacy admin status into the request so admin-scoped
	// handlers can gate on it (the console-CRUD authorization; the PDP authorizes
	// asset/PAM operations separately). Non-admin ⇒ false ⇒ admin handlers 403.
	req = req.WithContext(apihttp.WithAdmin(req.Context(), c.IsAdmin()))

	req.URL.Path = strings.TrimPrefix(req.URL.Path, boot.APIBasePath)
	handler.ServeHTTP(c.Ctx.ResponseWriter, req)
}

// parseSessionUUID reads a UUID stored as a string in the framework session.
func parseSessionUUID(v interface{}) (uuid.UUID, bool) {
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// bridgeDomainSession establishes a new-model auth_session for a just-completed
// interactive login and binds its ids into the framework session, so the control-
// plane API can resolve the session (pacote 011, T-004b). It is resolve-only: a
// user without a provisioned domain identity gets no binding (the domain API stays
// fail-closed for them), and a bridge failure is logged, never fatal to the login
// that already succeeded.
//
// The factors are derived conservatively from what the login proves: password
// always, plus a second factor when the user has MFA enabled (so it reached this
// point through MFA verification). It never over-claims assurance (INV-8);
// precise per-factor threading (e.g. WebAuthn→L3) is a later refinement.
func (c *ApiController) bridgeDomainSession(user *object.User) {
	methods := []domain.FactorType{domain.FactorPassword}
	if user.IsMfaEnabled() {
		methods = append(methods, domain.FactorTOTP)
	}

	id, sid, established, err := boot.BridgeLogin(c.Ctx.Request.Context(), user.Email, methods, time.Now())
	if err != nil {
		util.LogWarning(c.Ctx, "T-004b: ponte de sessão de domínio falhou para [%s]: %v", user.GetId(), err)
		return
	}
	if established {
		_ = c.SetSession(boot.SessionKeyIdentityID, id.String())
		_ = c.SetSession(boot.SessionKeyAuthSession, sid.String())
	}
}
