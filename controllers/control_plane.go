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

	"github.com/casdoor/casdoor/internal/boot"
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
func (c *RootController) HandleControlPlane() {
	if c.GetSessionUsername() == "" {
		c.Ctx.ResponseWriter.WriteHeader(http.StatusUnauthorized)
		return
	}

	handler := boot.APIHandler()
	if handler == nil {
		// The composition root has not mounted the API — refuse, do not fall open.
		c.Ctx.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	path := c.Ctx.Request.URL.Path
	c.Ctx.Request.URL.Path = strings.TrimPrefix(path, boot.APIBasePath)
	handler.ServeHTTP(c.Ctx.ResponseWriter, c.Ctx.Request)
}
