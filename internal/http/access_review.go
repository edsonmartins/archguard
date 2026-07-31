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
)

// AssetReviewer answers "who has effective access to this asset, and how"
// (postgres.PostgresPDP implements it via ReviewAsset). The handler depends on this
// narrow port, not on the adapter.
type AssetReviewer interface {
	ReviewAsset(ctx context.Context, assetRef string) ([]domain.AccessReviewEntry, error)
}

// AccessReviewHandler serves GET /access/review?asset=<id>: the effective access to
// an asset — each membership that can reach it and the ORIGIN of that access (direct,
// inherited from an ancestor group, or an active grant). It is the access-review
// (certification campaign) view (008 T-012). Admin operation (RequireAdmin gates it);
// the asset's tenant is the session's active org (never the request, INV-1).
//
// A PDP that cannot answer (ErrPDPUnavailable) is reported as 503 — the review could
// not run — NEVER as "no one has access" (fail-closed distinguishes "denied" from
// "could not decide").
type AccessReviewHandler struct {
	reviewer AssetReviewer
}

// NewAccessReviewHandler builds the handler over an asset reviewer.
func NewAccessReviewHandler(reviewer AssetReviewer) *AccessReviewHandler {
	return &AccessReviewHandler{reviewer: reviewer}
}

type reviewEntry struct {
	MembershipRef string   `json:"membership_ref"`
	Origins       []string `json:"origins"`
	Justification string   `json:"justification,omitempty"`
}

func (h *AccessReviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusConflict, "nenhum tenant ativo na sessão")
		return
	}
	asset := r.URL.Query().Get("asset")
	if asset == "" {
		writeError(w, http.StatusBadRequest, "asset é obrigatório")
		return
	}

	assetRef := domain.Qualify(*session.OrganizationID, domain.TypeAsset, asset)
	entries, err := h.reviewer.ReviewAsset(r.Context(), assetRef)
	if err != nil {
		writeAccessError(w, err) // 503 em ErrPDPUnavailable — nunca "ninguém tem acesso"
		return
	}

	items := make([]reviewEntry, 0, len(entries))
	for _, e := range entries {
		origins := make([]string, 0, len(e.Origins))
		for _, o := range e.Origins {
			origins = append(origins, string(o))
		}
		items = append(items, reviewEntry{
			MembershipRef: e.Subject,
			Origins:       origins,
			Justification: e.Justification,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"asset_id": asset, "entries": items})
}
