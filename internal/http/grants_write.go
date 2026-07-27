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
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// ErrGrantNotFound lets the handler answer 404 for an absent grant without importing
// the postgres package (boot maps the store's sentinel to this).
var ErrGrantNotFound = errors.New("concessão não encontrada")

// ErrGrantNotActive lets the handler answer 409 when the grant is not in a revocable
// (active) state — already revoked or expired (boot maps the domain transition error).
var ErrGrantNotActive = errors.New("concessão não está ativa")

// GrantRevoker revokes a privileged grant on behalf of an actor (boot composes it over
// postgres.PrivilegedAccessService + the audit writer).
type GrantRevoker interface {
	RevokeGrant(ctx context.Context, actor RevokeActor, organizationID, grantID uuid.UUID) error
}

// GrantRevokeHandler serves POST /grants/revoke {"grant_id":"…"}: it revokes an active
// privileged grant of the caller's ACTIVE tenant, cascade-revoking the grant's derived
// sessions. It is an L3 administration write (the mount gates it with the assurance
// pipeline at L3 — requiring step-up — plus the admin gate). The acting identity and the
// active organization are read from the session, never the request (INV-1/INV-5). Thin (§6).
type GrantRevokeHandler struct {
	revoker GrantRevoker
}

// NewGrantRevokeHandler builds the handler over a revoker.
func NewGrantRevokeHandler(revoker GrantRevoker) *GrantRevokeHandler {
	return &GrantRevokeHandler{revoker: revoker}
}

func (h *GrantRevokeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	var body struct {
		GrantID string `json:"grant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.GrantID == "" {
		writeError(w, http.StatusBadRequest, "grant_id obrigatório")
		return
	}
	grantID, err := uuid.Parse(body.GrantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "grant_id inválido")
		return
	}

	actor := RevokeActor{
		IdentityID:   session.IdentityID,
		MembershipID: session.MembershipID,
		SessionID:    session.ID,
	}
	err = h.revoker.RevokeGrant(r.Context(), actor, *session.OrganizationID, grantID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
	case errors.Is(err, ErrGrantNotFound):
		writeError(w, http.StatusNotFound, "concessão não encontrada neste tenant")
	case errors.Is(err, ErrGrantNotActive):
		writeError(w, http.StatusConflict, "concessão não está ativa (já revogada ou expirada)")
	default:
		// Any other failure (including a failed audit write) is fail-closed: the
		// revocation did not commit (I-5.4).
		writeError(w, http.StatusInternalServerError, "não foi possível revogar a concessão")
	}
}
