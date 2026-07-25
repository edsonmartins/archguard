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

// ErrMembershipNotFound lets the handler answer 404 for an absent target without
// importing the postgres package (boot maps the store's sentinel to this).
var ErrMembershipNotFound = errors.New("membership não encontrado")

// RevokeActor identifies who is performing a revocation — the caller's own session
// identity, membership and session, used to audit the mutation by a named actor.
type RevokeActor struct {
	IdentityID   uuid.UUID
	MembershipID *uuid.UUID
	SessionID    uuid.UUID
}

// MembershipRevoker revokes a membership on behalf of an actor (boot composes it
// over postgres.MembershipRevoker + the audit writer).
type MembershipRevoker interface {
	RevokeMembership(ctx context.Context, actor RevokeActor, organizationID, membershipID uuid.UUID) (sessionsEnded int, err error)
}

// MembershipRevokeHandler serves POST /memberships/revoke {"membership_id":"…"}:
// it revokes a membership of the caller's ACTIVE tenant and ends the member's
// sessions. It is an L2 administration write (the mount gates it with the assurance
// pipeline at L2 — requiring step-up — plus the admin gate). The acting identity is
// read from the session, never the request. Thin (§6).
type MembershipRevokeHandler struct {
	revoker MembershipRevoker
}

// NewMembershipRevokeHandler builds the handler over a revoker.
func NewMembershipRevokeHandler(revoker MembershipRevoker) *MembershipRevokeHandler {
	return &MembershipRevokeHandler{revoker: revoker}
}

func (h *MembershipRevokeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		MembershipID string `json:"membership_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MembershipID == "" {
		writeError(w, http.StatusBadRequest, "membership_id obrigatório")
		return
	}
	membershipID, err := uuid.Parse(body.MembershipID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_id inválido")
		return
	}

	actor := RevokeActor{
		IdentityID:   session.IdentityID,
		MembershipID: session.MembershipID,
		SessionID:    session.ID,
	}
	sessions, err := h.revoker.RevokeMembership(r.Context(), actor, *session.OrganizationID, membershipID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"revoked": true, "sessions_ended": sessions})
	case errors.Is(err, ErrMembershipNotFound):
		writeError(w, http.StatusNotFound, "membership não encontrado neste tenant")
	default:
		// Any other failure (including a failed audit write) is fail-closed: the
		// revocation did not commit (I-5.4).
		writeError(w, http.StatusInternalServerError, "não foi possível revogar o membership")
	}
}
