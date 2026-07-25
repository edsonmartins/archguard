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

// TenantMembershipLister lists the memberships of one tenant
// (postgres.TenantMembershipLister implements it). The handler depends on this
// narrow port, not on the adapter.
type TenantMembershipLister interface {
	ListInTenant(ctx context.Context, organizationID uuid.UUID) ([]domain.Membership, error)
}

// MembershipsHandler serves GET /memberships: the roster of the caller's ACTIVE
// tenant — an administrator view (RequireAdmin gates it at the mount). It reads the
// active organization from the injected session, never from the request, so the
// roster is always the caller's own tenant. Thin (CLAUDE.md §6). Personal data
// (name, e-mail) is not included — only identifiers and status (I-3.2); name
// enrichment needs the key custodian and is deferred.
type MembershipsHandler struct {
	lister TenantMembershipLister
}

// NewMembershipsHandler builds the handler over a tenant membership lister.
func NewMembershipsHandler(lister TenantMembershipLister) *MembershipsHandler {
	return &MembershipsHandler{lister: lister}
}

type membershipItem struct {
	MembershipID string `json:"membership_id"`
	IdentityID   string `json:"identity_id"`
	Status       string `json:"status"`
}

func (h *MembershipsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	if session.OrganizationID == nil {
		// No active tenant (pending selection): there is no roster to list.
		writeError(w, http.StatusConflict, "nenhum tenant ativo na sessão")
		return
	}

	memberships, err := h.lister.ListInTenant(r.Context(), *session.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar os membros")
		return
	}

	items := make([]membershipItem, 0, len(memberships))
	for _, m := range memberships {
		items = append(items, membershipItem{
			MembershipID: m.ID.String(),
			IdentityID:   m.IdentityID.String(),
			Status:       string(m.Status),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"memberships": items})
}
