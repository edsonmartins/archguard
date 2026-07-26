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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// TenantSwitcher moves the caller's ACTIVE session to another of its tenants,
// reissuing the token generation (design 002 §"Sessão e tenant ativo"). It returns
// the new session context on success; denials come back as DOMAIN errors that the
// handler maps to status — the distinction denied×error is preserved (a step-up
// need or a foreign membership is a DECISION, not a failure).
type TenantSwitcher interface {
	Switch(ctx context.Context, session *domain.AuthSession, targetOrganizationID uuid.UUID) (*domain.AuthSession, error)
}

// ErrDestNotMember: o chamador não tem membership ATIVO no tenant de destino — a
// troca é negada (403), nunca um erro genérico.
var ErrDestNotMember = errors.New("session: chamador não é membro ativo do tenant de destino")

// SessionSwitchHandler serves POST /api/v1/session/tenant: troca o tenant ativo da
// sessão. Handler fino (CLAUDE.md §6) — resolve a sessão que o middleware injetou,
// lê o destino do corpo e delega ao TenantSwitcher; a política do destino decide o
// step-up (401 RFC 9470), não o nível de operação do middleware.
type SessionSwitchHandler struct{ switcher TenantSwitcher }

// NewSessionSwitchHandler builds the handler over the switch capability.
func NewSessionSwitchHandler(s TenantSwitcher) *SessionSwitchHandler {
	return &SessionSwitchHandler{switcher: s}
}

type switchRequest struct {
	OrganizationID string `json:"organization_id"`
}

func (h *SessionSwitchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		// Defensivo: o middleware autoriza antes; sessão ausente é fail-closed.
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}

	var req switchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	orgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "organization_id inválido")
		return
	}

	next, err := h.switcher.Switch(r.Context(), session, orgID)
	if err != nil {
		writeSwitchError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionContextBody(next))
}

// writeSwitchError mapeia os erros de domínio para status, preservando denied×error.
func writeSwitchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrStepUpRequired):
		// RFC 9470: a troca exige um fator mais forte — desafio de step-up.
		w.Header().Set("WWW-Authenticate",
			`Bearer error="insufficient_user_authentication", error_description="o tenant de destino exige um fator mais forte"`)
		writeError(w, http.StatusUnauthorized, "insufficient_user_authentication: o tenant de destino exige step-up")
	case errors.Is(err, domain.ErrSameTenant):
		writeError(w, http.StatusConflict, "o tenant de destino já é o ativo")
	case errors.Is(err, ErrDestNotMember),
		errors.Is(err, domain.ErrForeignMembership),
		errors.Is(err, domain.ErrMembershipNotSelectable):
		writeError(w, http.StatusForbidden, "tenant de destino inválido para o chamador")
	case errors.Is(err, domain.ErrSessionRevoked),
		errors.Is(err, domain.ErrTenantSelectionRequired):
		writeError(w, http.StatusUnauthorized, "sessão não elegível para troca")
	case errors.Is(err, domain.ErrDestinationPolicyUnavailable),
		errors.Is(err, domain.ErrSwitchAuditUnavailable):
		// INV-6: política/auditoria indisponível ⇒ negação (fail-closed), nunca troca.
		writeError(w, http.StatusServiceUnavailable, "troca de tenant indisponível (fail-closed)")
	default:
		writeError(w, http.StatusInternalServerError, "falha ao trocar de tenant")
	}
}

// sessionContextBody constrói o mesmo corpo do GET /session para a sessão pós-troca
// (identificadores e garantia apenas — sem dado pessoal, I-3.2).
func sessionContextBody(s *domain.AuthSession) sessionContextResponse {
	resp := sessionContextResponse{
		IdentityID: s.IdentityID.String(),
		Status:     string(s.Status),
		ProvenAAL:  string(s.ProvenAAL),
	}
	if s.OrganizationID != nil {
		resp.OrganizationID = s.OrganizationID.String()
	}
	if s.MembershipID != nil {
		resp.MembershipID = s.MembershipID.String()
	}
	if !s.AuthTime.IsZero() {
		resp.AuthTime = s.AuthTime.Unix()
	}
	for _, m := range s.AuthMethods {
		resp.AMR = append(resp.AMR, string(m))
	}
	return resp
}
