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
	"net/http"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// TokenVerifier parses and verifies a token's signature and returns its claims —
// the oidc.Signer implements it. Expiry is NOT enforced here; liveness is decided
// by the introspection handler composing this with the session-liveness check.
type TokenVerifier interface {
	Verify(token string) (domain.OIDCClaims, error)
}

// IntrospectionHandler serves POST /introspect (RFC 7662). It verifies the
// presented token, checks whether its session is still live, and returns the
// introspection response. An UNVERIFIABLE token (bad signature, unknown kid) or a
// revoked/expired one yields `active: false` with no claims — a component learns
// only whether the token is currently usable, and a revoked session introspects
// as inactive before the access token expires (the revocation propagation for
// components without back-channel logout, T-010).
type IntrospectionHandler struct {
	verifier TokenVerifier
	liveness domain.SessionLiveness
	now      func() time.Time
}

// NewIntrospectionHandler builds the handler over a verifier and a liveness
// checker.
func NewIntrospectionHandler(verifier TokenVerifier, liveness domain.SessionLiveness) *IntrospectionHandler {
	return &IntrospectionHandler{verifier: verifier, liveness: liveness, now: time.Now}
}

func (h *IntrospectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "form inválido")
		return
	}
	token := r.PostForm.Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token ausente")
		return
	}

	// A token we cannot verify is simply not active — never an error to the caller
	// (RFC 7662 §2.2: only active:false is disclosed).
	claims, err := h.verifier.Verify(token)
	if err != nil {
		writeJSON(w, http.StatusOK, domain.IntrospectionResponse{Active: false})
		return
	}
	// Liveness of the session behind the token. Fail-closed: an error checking
	// liveness is treated as NOT live, so an unverifiable session never reads as
	// active.
	live, lerr := h.liveness.Live(r.Context(), claims.Organization, claims.SessionID)
	if lerr != nil {
		live = false
	}
	writeJSON(w, http.StatusOK, domain.BuildIntrospection(claims, live, h.now()))
}
