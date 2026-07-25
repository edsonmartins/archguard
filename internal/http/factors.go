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

// ErrNoPendingEnrollment mirrors the boot-layer sentinel so the handler can map a
// missing pending enrollment to a distinct status without importing boot.
var ErrNoPendingEnrollment = errors.New("nenhuma inscrição TOTP pendente")

// TOTPEnroller drives the TOTP enrollment ceremony (boot composes it over the
// totp.Service, the credential store and the vault). The handler depends on this
// narrow port.
type TOTPEnroller interface {
	BeginTOTP(ctx context.Context, identityID uuid.UUID) (provisioningURI string, err error)
	FinishTOTP(ctx context.Context, identityID uuid.UUID, code string) error
}

// FactorsHandler serves the factor-enrollment endpoints. It enrolls a factor for
// the CALLER's identity (read from the injected session, never the request), so a
// caller cannot enroll a factor for someone else. Thin (§6).
type FactorsHandler struct {
	enroller TOTPEnroller
}

// NewFactorsHandler builds the handler over an enroller.
func NewFactorsHandler(enroller TOTPEnroller) *FactorsHandler {
	return &FactorsHandler{enroller: enroller}
}

// BeginTOTP serves POST /factors/totp/begin: it generates a seed and returns the
// provisioning URI for the caller to add to their authenticator.
func (h *FactorsHandler) BeginTOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	uri, err := h.enroller.BeginTOTP(r.Context(), session.IdentityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível iniciar a inscrição do fator")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"provisioning_uri": uri})
}

// FinishTOTP serves POST /factors/totp/verify {"code":"123456"}: it confirms the
// seed and registers the factor.
func (h *FactorsHandler) FinishTOTP(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusBadRequest, "código de confirmação obrigatório")
		return
	}

	err := h.enroller.FinishTOTP(r.Context(), session.IdentityID, body.Code)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]bool{"enrolled": true})
	case errors.Is(err, ErrNoPendingEnrollment):
		writeError(w, http.StatusConflict, "nenhuma inscrição pendente — reinicie a inscrição")
	default:
		// The common case is a wrong code (client-fixable); the seed is not persisted.
		writeError(w, http.StatusBadRequest, "não foi possível confirmar o fator (código inválido)")
	}
}
