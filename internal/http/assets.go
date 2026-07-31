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

// AssetCatalog is the tenant-scoped asset catalog the handler depends on
// (postgres.AssetCatalog implements it). Both operations are confined to the
// session's tenant — the org is resolved from the session, never the request (INV-1).
type AssetCatalog interface {
	ListInTenant(ctx context.Context, orgID uuid.UUID) ([]domain.Asset, error)
	CreateInTenant(ctx context.Context, orgID uuid.UUID, a domain.Asset) error
}

// AssetsHandler serves GET/POST /api/v1/assets — the tenant's asset catalog (pacote
// 007 M4, T-026). GET lists; POST registers a new asset (kind + name required; parent
// group and owner optional). Admin operation (RequireAdmin gates it). The org is
// always the session's active tenant.
type AssetsHandler struct {
	catalog AssetCatalog
}

// NewAssetsHandler builds the handler over the tenant asset catalog.
func NewAssetsHandler(catalog AssetCatalog) *AssetsHandler {
	return &AssetsHandler{catalog: catalog}
}

type assetItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	ExternalRef string `json:"external_ref,omitempty"`
	ParentGroup string `json:"parent_group_id,omitempty"`
	Owner       string `json:"owner_membership_id,omitempty"`
}

type createAssetBody struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	ExternalRef string `json:"external_ref"`
	ParentGroup string `json:"parent_group_id"`
	Owner       string `json:"owner_membership_id"`
}

func (h *AssetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *AssetsHandler) list(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	assets, err := h.catalog.ListInTenant(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar os ativos")
		return
	}
	items := make([]assetItem, 0, len(assets))
	for _, a := range assets {
		items = append(items, toAssetItem(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": items})
}

func (h *AssetsHandler) create(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) {
	var body createAssetBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	parent, err := optionalUUID(body.ParentGroup)
	if err != nil {
		writeError(w, http.StatusBadRequest, "parent_group_id inválido")
		return
	}
	owner, err := optionalUUID(body.Owner)
	if err != nil {
		writeError(w, http.StatusBadRequest, "owner_membership_id inválido")
		return
	}
	asset, err := domain.NewAsset(orgID, body.Kind, body.Name, body.ExternalRef, parent, owner)
	if err != nil {
		writeError(w, http.StatusBadRequest, "kind e name são obrigatórios")
		return
	}
	if err := h.catalog.CreateInTenant(r.Context(), orgID, asset); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível registrar o ativo")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"asset": toAssetItem(asset)})
}

func toAssetItem(a domain.Asset) assetItem {
	it := assetItem{ID: a.ID.String(), Kind: a.Kind, Name: a.Name, ExternalRef: a.ExternalRef}
	if a.ParentGroupID != nil {
		it.ParentGroup = a.ParentGroupID.String()
	}
	if a.OwnerMembershipID != nil {
		it.Owner = a.OwnerMembershipID.String()
	}
	return it
}

// optionalUUID parses an optional uuid field: "" → nil; a malformed value → error.
func optionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
