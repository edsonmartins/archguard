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

// AssetAccessCatalog is the tenant-scoped access-assignment catalog the handler
// depends on (postgres.AssetAccessCatalog implements it). Both operations are confined
// to the session's tenant (the org is resolved from the session, never the request).
type AssetAccessCatalog interface {
	ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.AssetAccessAssignment, error)
	CreateInTenant(ctx context.Context, orgID uuid.UUID, a domain.AssetAccessAssignment) error
}

// AssetAccessHandler serves GET/POST /api/v1/access-assignments — the granular access
// assignments of the tenant (pacote 007 M4, T-029). GET lists; POST grants a membership
// operator/auditor over an asset or asset_group. Admin operation (RequireAdmin gates it).
type AssetAccessHandler struct {
	catalog AssetAccessCatalog
}

// NewAssetAccessHandler builds the handler over the tenant access catalog.
func NewAssetAccessHandler(catalog AssetAccessCatalog) *AssetAccessHandler {
	return &AssetAccessHandler{catalog: catalog}
}

type accessAssignmentItem struct {
	ID         string `json:"id"`
	SubjectID  string `json:"subject_id"`
	Relation   string `json:"relation"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

type createAssignmentBody struct {
	SubjectID  string `json:"subject_id"`
	Relation   string `json:"relation"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

func (h *AssetAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *AssetAccessHandler) list(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	items, err := h.catalog.ListInTenant(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar as atribuições")
		return
	}
	out := make([]accessAssignmentItem, 0, len(items))
	for _, a := range items {
		out = append(out, toAssignmentItem(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": out})
}

func (h *AssetAccessHandler) create(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	var body createAssignmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	subjectID, err := uuid.Parse(body.SubjectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "subject_id inválido")
		return
	}
	objectID, err := uuid.Parse(body.ObjectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "object_id inválido")
		return
	}
	assignment, err := domain.NewAssetAccessAssignment(orgID, subjectID, body.Relation, domain.ObjectType(body.ObjectType), objectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "relação (operator/auditor) e objeto (asset/asset_group) são obrigatórios e válidos")
		return
	}
	if err := h.catalog.CreateInTenant(r.Context(), orgID, assignment); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível criar a atribuição")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"assignment": toAssignmentItem(assignment)})
}

func toAssignmentItem(a domain.AssetAccessAssignment) accessAssignmentItem {
	return accessAssignmentItem{
		ID:         a.ID.String(),
		SubjectID:  a.SubjectID.String(),
		Relation:   a.Relation,
		ObjectType: string(a.ObjectType),
		ObjectID:   a.ObjectID.String(),
	}
}
