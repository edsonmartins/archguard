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
	"net/http"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// MembershipLister lists an identity's memberships across tenants
// (postgres.MembershipReader implements it). The handler depends on this narrow
// port, not on the adapter.
type MembershipLister interface {
	ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]domain.Membership, error)
}

// TenantsHandler serves GET /tenants: the tenants the authenticated caller can act
// in — the data the console's tenant selector needs. It reads the caller's identity
// from the injected session, never from the request, so one caller cannot list
// another's tenants. Thin (CLAUDE.md §6): resolve, list, encode.
type TenantsHandler struct {
	lister MembershipLister
}

// NewTenantsHandler builds the handler over a membership lister.
func NewTenantsHandler(lister MembershipLister) *TenantsHandler {
	return &TenantsHandler{lister: lister}
}

// tenantItem is one selectable tenant. active marks the session's current tenant.
type tenantItem struct {
	MembershipID   string `json:"membership_id"`
	OrganizationID string `json:"organization_id"`
	Status         string `json:"status"`
	Active         bool   `json:"active"`
}

func (h *TenantsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}

	memberships, err := h.lister.ListByIdentity(r.Context(), session.IdentityID)
	if err != nil {
		// A list that could not be produced is fail-closed: never serve a partial
		// or empty list as authoritative.
		writeError(w, http.StatusInternalServerError, "não foi possível listar os tenants")
		return
	}

	items := make([]tenantItem, 0, len(memberships))
	for _, m := range memberships {
		items = append(items, tenantItem{
			MembershipID:   m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			Status:         string(m.Status),
			Active:         session.OrganizationID != nil && *session.OrganizationID == m.OrganizationID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": items})
}
