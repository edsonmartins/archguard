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

// ErrNoStrongFactor is returned when the caller has no strong factor enrolled to
// step up with — the client must enroll one first.
var ErrNoStrongFactor = errors.New("nenhum fator forte inscrito")

// ErrStepUpDenied is a step-up DENIAL (wrong factor proof) — distinct from an
// infrastructure failure, so the handler answers 401 (auth failed), not 503.
var ErrStepUpDenied = errors.New("prova de fator inválida")

// StepUpService raises a session's assurance after a fresh factor proof (boot
// composes it over the totp.Service, the credential store and the session store).
type StepUpService interface {
	StepUpTOTP(ctx context.Context, identityID, sessionID uuid.UUID, code string, now time.Time) (domain.AAL, error)
}

// StepUpHandler serves the step-up ceremony. It is exempt from the assurance
// middleware (operation_catalog: auth.stepup) — it is how the level is raised, so
// gating it on a level would be circular; the factor proof IS the control. It steps
// up the CALLER's own session (identity and session id from the injected session,
// never the request), so one caller cannot elevate another's session.
type StepUpHandler struct {
	svc StepUpService
	now func() time.Time
}

// NewStepUpHandler builds the handler over a step-up service.
func NewStepUpHandler(svc StepUpService) *StepUpHandler {
	return &StepUpHandler{svc: svc, now: time.Now}
}

// TOTP serves POST /stepup/totp {"code":"123456"}: on a valid code it raises the
// session to AAL2 and returns the new level.
func (h *StepUpHandler) TOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeError(w, http.StatusBadRequest, "código obrigatório")
		return
	}

	aal, err := h.svc.StepUpTOTP(r.Context(), session.IdentityID, session.ID, body.Code, h.now())
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"aal": string(aal)})
	case errors.Is(err, ErrNoStrongFactor):
		writeError(w, http.StatusConflict, "nenhum fator TOTP inscrito — inscreva um fator antes de elevar")
	case errors.Is(err, ErrStepUpDenied):
		writeError(w, http.StatusUnauthorized, "código inválido")
	default:
		// A vault/database failure is fail-closed: the level was not raised.
		writeError(w, http.StatusServiceUnavailable, "step-up não pôde ser concluído")
	}
}
