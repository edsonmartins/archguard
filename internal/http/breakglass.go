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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ErrBreakglassChannelUnavailable lets the handler answer 503 when the tenant has no
// notification channel to announce the emergency access on — fail-closed (boot maps the
// domain sentinel to this).
var ErrBreakglassChannelUnavailable = errors.New("nenhum canal de notificação disponível")

// ErrBreakglassInvalid lets the handler answer 422 when the domain rejects the request
// (missing justification/incident, invalid window) — boot maps the domain sentinel.
var ErrBreakglassInvalid = errors.New("solicitação de break-glass inválida")

// ErrBreakglassNeedsWebAuthn lets the handler answer 403 when the caller's step-up is not
// phishing-resistant (TOTP does not qualify for break-glass). The L3 pipeline gate already
// requires WebAuthn; this is the defense-in-depth denial if it was bypassed.
var ErrBreakglassNeedsWebAuthn = errors.New("break-glass exige step-up WebAuthn")

// BreakglassRequester opens a break-glass request on behalf of an actor (boot composes it
// over postgres.BreakglassOrchestrator + the notifier + the audit writer). provenAAL and
// phishingResistant come from the caller's session — the request advances the grant past
// the step-up gate (requested → awaiting_approval) with them.
type BreakglassRequester interface {
	RequestBreakglass(ctx context.Context, actor RevokeActor, provenAAL domain.AAL, phishingResistant bool, organizationID uuid.UUID, target domain.GrantTarget, justification, incidentRef string, notBefore, expiresAt time.Time) error
}

// BreakglassRequestHandler serves POST /breakglass/request: it opens an emergency-access
// (break-glass) grant for the CALLER themselves (the subject is the requesting operator)
// over an opaque target, with a mandatory justification and incident reference. It is an
// L3 write (the mount gates it with the assurance pipeline at L3 — requiring step-up —
// plus the admin gate; L3 is denied outright in the dev profile). The subject membership
// and the active organization are read from the session, never the request (INV-1/INV-5).
// It is fail-closed on the notification channel (503) and on the audit (500). Thin (§6).
type BreakglassRequestHandler struct {
	requester BreakglassRequester
}

// NewBreakglassRequestHandler builds the handler over a requester.
func NewBreakglassRequestHandler(requester BreakglassRequester) *BreakglassRequestHandler {
	return &BreakglassRequestHandler{requester: requester}
}

func (h *BreakglassRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if session.MembershipID == nil {
		writeError(w, http.StatusConflict, "sessão sem membership ativo para receber o acesso")
		return
	}
	var body struct {
		TargetType    string `json:"target_type"`
		TargetID      string `json:"target_id"`
		TargetScope   string `json:"target_scope"`
		Justification string `json:"justification"`
		IncidentRef   string `json:"incident_ref"`
		ExpiresAt     int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if body.TargetType == "" || body.TargetID == "" || body.Justification == "" || body.IncidentRef == "" {
		writeError(w, http.StatusBadRequest, "target_type, target_id, justification e incident_ref são obrigatórios")
		return
	}
	notBefore := time.Now()
	if body.ExpiresAt <= 0 {
		writeError(w, http.StatusBadRequest, "expires_at (Unix) obrigatório")
		return
	}
	expiresAt := time.Unix(body.ExpiresAt, 0)
	if !expiresAt.After(notBefore) {
		writeError(w, http.StatusBadRequest, "expires_at deve ser no futuro")
		return
	}
	target := domain.GrantTarget{Type: body.TargetType, ID: body.TargetID, Scope: body.TargetScope}

	actor := RevokeActor{
		IdentityID:   session.IdentityID,
		MembershipID: session.MembershipID,
		SessionID:    session.ID,
	}
	err := h.requester.RequestBreakglass(r.Context(), actor, session.ProvenAAL, session.PhishingResistant(), *session.OrganizationID, target, body.Justification, body.IncidentRef, notBefore, expiresAt)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, map[string]any{"requested": true})
	case errors.Is(err, ErrBreakglassNeedsWebAuthn):
		writeError(w, http.StatusForbidden, "break-glass exige step-up com fator resistente a phishing (WebAuthn)")
	case errors.Is(err, ErrBreakglassChannelUnavailable):
		// Fail-closed: an emergency access that cannot be announced is denied (T-013).
		writeError(w, http.StatusServiceUnavailable, "nenhum canal de notificação disponível para alertar — solicitação negada")
	case errors.Is(err, ErrBreakglassInvalid):
		writeError(w, http.StatusUnprocessableEntity, "solicitação de break-glass inválida (justificativa/incidente/janela)")
	default:
		// Any other failure (including a failed audit write) is fail-closed: the request
		// did not persist (I-5.4).
		writeError(w, http.StatusInternalServerError, "não foi possível registrar a solicitação de break-glass")
	}
}
