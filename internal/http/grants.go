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

// GrantLister lists a tenant's active privileged grants
// (postgres.TenantGrantLister implements it).
type GrantLister interface {
	ListActive(ctx context.Context, organizationID uuid.UUID) ([]domain.PrivilegedGrant, error)
}

// GrantsHandler serves GET /grants: the active privileged grants of the caller's
// ACTIVE tenant — who currently holds privileged access to what, for how long. An
// administration view (RequireAdmin gates it at the mount). It reads the active
// organization from the injected session, never from the request. Thin (§6).
type GrantsHandler struct {
	lister GrantLister
}

// NewGrantsHandler builds the handler over a grant lister.
func NewGrantsHandler(lister GrantLister) *GrantsHandler {
	return &GrantsHandler{lister: lister}
}

// grantItem is one active grant. Target is the opaque asset reference (there is no
// asset catalog to resolve it — RFC-0004 §9 / M4); the console shows it as-is.
type grantItem struct {
	GrantID             string `json:"grant_id"`
	SubjectMembershipID string `json:"subject_membership_id"`
	TargetType          string `json:"target_type"`
	TargetID            string `json:"target_id"`
	TargetScope         string `json:"target_scope,omitempty"`
	Origin              string `json:"origin"`
	Status              string `json:"status"`
	NotBefore           int64  `json:"not_before"`
	ExpiresAt           int64  `json:"expires_at"`
}

func (h *GrantsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	grants, err := h.lister.ListActive(r.Context(), *session.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar as concessões")
		return
	}

	items := make([]grantItem, 0, len(grants))
	for _, g := range grants {
		items = append(items, grantItem{
			GrantID:             g.ID.String(),
			SubjectMembershipID: g.SubjectMembershipID.String(),
			TargetType:          g.Target.Type,
			TargetID:            g.Target.ID,
			TargetScope:         g.Target.Scope,
			Origin:              string(g.Origin),
			Status:              string(g.Status),
			NotBefore:           g.NotBefore.Unix(),
			ExpiresAt:           g.ExpiresAt.Unix(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"grants": items})
}
