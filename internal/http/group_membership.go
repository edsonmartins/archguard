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
	"net/http"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// GroupMembershipCatalog is the tenant-scoped group-binding catalog the handler depends on
// (postgres.GroupMembershipCatalog implements it). Confined to the session's tenant (INV-1).
type GroupMembershipCatalog interface {
	ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.GroupMembership, error)
	CreateInTenant(ctx context.Context, orgID uuid.UUID, g domain.GroupMembership) error
}

// GroupMembershipHandler serves GET/POST /api/v1/group-memberships — the membership↔access-
// group bindings of the tenant (pacote 007 M4, T-029 D1). GET lists; POST binds a membership
// to an access group. Admin operation (RequireAdmin gates it).
type GroupMembershipHandler struct {
	catalog GroupMembershipCatalog
}

// NewGroupMembershipHandler builds the handler over the tenant group catalog.
func NewGroupMembershipHandler(catalog GroupMembershipCatalog) *GroupMembershipHandler {
	return &GroupMembershipHandler{catalog: catalog}
}

type groupMembershipItem struct {
	ID           string `json:"id"`
	GroupID      string `json:"group_id"`
	MembershipID string `json:"membership_id"`
}

type createGroupMembershipBody struct {
	GroupID      string `json:"group_id"`
	MembershipID string `json:"membership_id"`
}

func (h *GroupMembershipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	if session.OrganizationID == nil {
		writeError(w, http.StatusConflict, "nenhum tenant ativo na sessão")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r, *session.OrganizationID)
	case http.MethodPost:
		h.create(w, r, *session.OrganizationID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
	}
}

func (h *GroupMembershipHandler) list(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	items, err := h.catalog.ListInTenant(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar os vínculos de grupo")
		return
	}
	out := make([]groupMembershipItem, 0, len(items))
	for _, g := range items {
		out = append(out, groupMembershipItem{ID: g.ID.String(), GroupID: g.GroupID.String(), MembershipID: g.MembershipID.String()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"group_memberships": out})
}

func (h *GroupMembershipHandler) create(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	var body createGroupMembershipBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	groupID, err := uuid.Parse(body.GroupID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "group_id inválido")
		return
	}
	membershipID, err := uuid.Parse(body.MembershipID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_id inválido")
		return
	}
	binding, err := domain.NewGroupMembership(orgID, groupID, membershipID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "group_id e membership_id são obrigatórios")
		return
	}
	if err := h.catalog.CreateInTenant(r.Context(), orgID, binding); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível criar o vínculo de grupo")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group_membership": groupMembershipItem{
		ID: binding.ID.String(), GroupID: binding.GroupID.String(), MembershipID: binding.MembershipID.String(),
	}})
}
