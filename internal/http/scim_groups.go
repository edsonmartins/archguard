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

package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// SCIMGroupProvisioner reconciles a provisioned group and its members within an
// organization, returning the resource id. Directory group membership is
// authoritative, but the implementation NEVER derives a privileged role from a
// group without an approved mapping (design 009); this handler is agnostic to that.
type SCIMGroupProvisioner interface {
	ProvisionGroup(ctx context.Context, orgID uuid.UUID, rec domain.GroupSyncRecord) (id string, err error)
}

// SCIMGroupHandler serves the SCIM 2.0 Groups endpoint (inbound, pacote 009 T-008).
// Thin: parse/validate → resolve tenant → delegate → render SCIM response.
type SCIMGroupHandler struct {
	provisioner SCIMGroupProvisioner
	org         OrgResolver
}

// NewSCIMGroupHandler builds the handler over a provisioner and a tenant resolver.
func NewSCIMGroupHandler(provisioner SCIMGroupProvisioner, org OrgResolver) *SCIMGroupHandler {
	return &SCIMGroupHandler{provisioner: provisioner, org: org}
}

// ServeHTTP handles POST /Groups (create).
func (h *SCIMGroupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeSCIMError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	orgID, ok := h.org(r)
	if !ok {
		writeSCIMError(w, http.StatusUnauthorized, "tenant não resolvido")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "corpo ilegível")
		return
	}
	group, err := domain.ParseSCIMGroup(body)
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := h.provisioner.ProvisionGroup(r.Context(), orgID, group.ToGroupRecord())
	if err != nil {
		writeSCIMError(w, http.StatusConflict, "provisionamento de grupo falhou")
		return
	}

	resource := group.ResponseGroup(id, "/scim/v2/Groups/"+id)
	w.Header().Set("Content-Type", "application/scim+json")
	w.Header().Set("Location", resource.Meta.Location)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resource)
}
