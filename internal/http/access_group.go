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

// AccessGroupCatalog is the tenant-scoped access-group catalog the handler depends on
// (postgres.AccessGroupCatalog implements it). Confined to the session's tenant (INV-1).
type AccessGroupCatalog interface {
	ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.AccessGroup, error)
	CreateInTenant(ctx context.Context, orgID uuid.UUID, g domain.AccessGroup) error
}

// AccessGroupHandler serves GET/POST /api/v1/access-groups — the tenant's access-group
// catalog (pacote 007 M4, D1 catálogo). GET lists; POST names a new group. Admin
// operation (RequireAdmin gates it).
type AccessGroupHandler struct {
	catalog AccessGroupCatalog
}

// NewAccessGroupHandler builds the handler over the tenant catalog.
func NewAccessGroupHandler(catalog AccessGroupCatalog) *AccessGroupHandler {
	return &AccessGroupHandler{catalog: catalog}
}

type accessGroupItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type createAccessGroupBody struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func (h *AccessGroupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *AccessGroupHandler) list(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	items, err := h.catalog.ListInTenant(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar os grupos")
		return
	}
	out := make([]accessGroupItem, 0, len(items))
	for _, g := range items {
		out = append(out, accessGroupItem{ID: g.ID.String(), Name: g.Name, DisplayName: g.DisplayName})
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

func (h *AccessGroupHandler) create(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	var body createAccessGroupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	group, err := domain.NewAccessGroup(orgID, body.Name, body.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "nome é obrigatório")
		return
	}
	if err := h.catalog.CreateInTenant(r.Context(), orgID, group); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível criar o grupo")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": accessGroupItem{
		ID: group.ID.String(), Name: group.Name, DisplayName: group.DisplayName,
	}})
}
