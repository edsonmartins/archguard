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
	"strconv"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// SCIMUserProvisioner creates (or reconciles) the identity + membership for a
// provisioned SCIM user within an organization, returning the resource id. Its
// implementation deduplicates by email_hash so a known e-mail never creates a
// second identity (T-009/T-012); this handler is agnostic to that.
type SCIMUserProvisioner interface {
	ProvisionUser(ctx context.Context, orgID uuid.UUID, rec domain.DirectorySyncRecord) (id string, err error)
}

// OrgResolver resolves the target organization of a SCIM request (from the path or
// the bearer token's tenant binding). ok=false ⇒ the request is not tenant-scoped.
type OrgResolver func(r *http.Request) (uuid.UUID, bool)

// SCIMUserHandler serves the SCIM 2.0 Users endpoint (inbound provisioning, pacote
// 009 T-007 / RFC-0007 §5.2). It is a THIN handler: it parses/validates the SCIM
// resource, resolves the tenant, delegates creation to the provisioner, and
// renders the SCIM response. No authorization or dedup logic lives here.
type SCIMUserHandler struct {
	provisioner SCIMUserProvisioner
	org         OrgResolver
}

// NewSCIMUserHandler builds the handler over a provisioner and a tenant resolver.
func NewSCIMUserHandler(provisioner SCIMUserProvisioner, org OrgResolver) *SCIMUserHandler {
	return &SCIMUserHandler{provisioner: provisioner, org: org}
}

// ServeHTTP handles POST /Users (create). Other methods are not implemented here.
func (h *SCIMUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	user, err := domain.ParseSCIMUser(body)
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := h.provisioner.ProvisionUser(r.Context(), orgID, user.ToSyncRecord())
	if err != nil {
		writeSCIMError(w, http.StatusConflict, "provisionamento falhou")
		return
	}

	resource := user.ResponseUser(id, "/scim/v2/Users/"+id)
	w.Header().Set("Content-Type", "application/scim+json")
	w.Header().Set("Location", resource.Meta.Location)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resource)
}

// scimErrorSchema is the SCIM 2.0 error message schema URN (RFC 7644 §3.12).
const scimErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"

// writeSCIMError renders a SCIM-format error. status is echoed as a string per the
// spec; detail must never carry a secret (it is derived from validation errors,
// which are non-sensitive).
func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schemas": []string{scimErrorSchema},
		"detail":  detail,
		"status":  strconv.Itoa(status),
	})
}
