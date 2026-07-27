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
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// StepUpWebAuthnService runs the WebAuthn assertion step-up ceremony (boot composes it
// over the webauthn.Service, the credential store and the session store). It is the
// phishing-resistant step-up — the only one that can satisfy an L3 operation (TOTP caps
// at AAL2). Reusing ErrNoStrongFactor/ErrStepUpDenied from the TOTP step-up.
type StepUpWebAuthnService interface {
	// BeginWebAuthn starts the assertion ceremony for the caller's session, returning the
	// options the browser runs `navigator.credentials.get` with.
	BeginWebAuthn(ctx context.Context, identityID, sessionID uuid.UUID) (options any, err error)
	// FinishWebAuthn validates the assertion and, on success, raises the session to the
	// phishing-resistant AAL of the authenticator used, returning the new level.
	FinishWebAuthn(ctx context.Context, identityID, sessionID uuid.UUID, assertion []byte, now time.Time) (domain.AAL, error)
}

// StepUpWebAuthnHandler serves the two-legged WebAuthn step-up. Like the TOTP step-up it
// is exempt from the assurance middleware (the factor proof IS the control) and steps up
// the CALLER's own session (identity and session id from the injected session, never the
// request). Thin (§6).
type StepUpWebAuthnHandler struct {
	svc StepUpWebAuthnService
	now func() time.Time
}

// NewStepUpWebAuthnHandler builds the handler over a step-up service.
func NewStepUpWebAuthnHandler(svc StepUpWebAuthnService) *StepUpWebAuthnHandler {
	return &StepUpWebAuthnHandler{svc: svc, now: time.Now}
}

// Begin serves POST /stepup/webauthn/begin: returns the assertion options for the caller's
// registered authenticators (the challenge is kept server-side until finish).
func (h *StepUpWebAuthnHandler) Begin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	options, err := h.svc.BeginWebAuthn(r.Context(), session.IdentityID, session.ID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, options)
	case errors.Is(err, ErrNoStrongFactor):
		writeError(w, http.StatusConflict, "nenhum fator WebAuthn inscrito — inscreva um fator antes de elevar")
	default:
		writeError(w, http.StatusServiceUnavailable, "step-up não pôde ser iniciado")
	}
}

// Finish serves POST /stepup/webauthn/finish: the browser posts the raw assertion; a valid
// assertion raises the session to the authenticator's (phishing-resistant) AAL.
func (h *StepUpWebAuthnHandler) Finish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadRequest, "asserção obrigatória")
		return
	}
	aal, err := h.svc.FinishWebAuthn(r.Context(), session.IdentityID, session.ID, body, h.now())
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"aal": string(aal)})
	case errors.Is(err, ErrNoStrongFactor):
		writeError(w, http.StatusConflict, "nenhum fator WebAuthn inscrito")
	case errors.Is(err, ErrStepUpDenied):
		writeError(w, http.StatusUnauthorized, "asserção inválida ou desafio expirado")
	default:
		// A database failure is fail-closed: the level was not raised.
		writeError(w, http.StatusServiceUnavailable, "step-up não pôde ser concluído")
	}
}
