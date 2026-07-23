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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// EndSession ends a session at ArchGuard and propagates back-channel logout to
// the components (the postgres+domain wiring implements it, composing
// LogoutPropagator with the client registry). It returns an error only when the
// LOCAL revocation failed (fail-closed); best-effort component notifications that
// failed are surfaced separately by the propagator's log/audit.
type EndSession interface {
	EndSession(ctx context.Context, sessionID uuid.UUID, sid string) error
}

// EndSessionHandler serves GET/POST /logout — RP-initiated logout (OIDC end
// session). It resolves the caller's session (login seam), revokes it and its
// derived sessions/tokens, propagates back-channel logout, and redirects to a
// validated post_logout_redirect_uri (or answers 204). Local revocation is
// fail-closed: if it fails, the handler answers 500 rather than pretend logout.
type EndSessionHandler struct {
	resolve  SessionResolver
	ender    EndSession
	registry *domain.ClientRegistry
	now      func() time.Time
}

// NewEndSessionHandler builds the handler.
func NewEndSessionHandler(resolve SessionResolver, ender EndSession, registry *domain.ClientRegistry) *EndSessionHandler {
	return &EndSessionHandler{resolve: resolve, ender: ender, registry: registry, now: time.Now}
}

func (h *EndSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := h.resolve.Session(r)
	if !ok || session == nil {
		// No session to end — nothing to do; the RP may still redirect the user.
		h.maybeRedirect(w, r)
		return
	}
	if err := h.ender.EndSession(r.Context(), session.ID, session.ID.String()); err != nil {
		// Fail-closed: an unpropagatable/unrevocable logout is a server error, not a
		// silent success.
		writeError(w, http.StatusInternalServerError, "não foi possível encerrar a sessão")
		return
	}
	h.maybeRedirect(w, r)
}

// maybeRedirect sends the user to a post_logout_redirect_uri IF it is registered
// for the given client (open-redirect guard); otherwise it answers 204.
func (h *EndSessionHandler) maybeRedirect(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("post_logout_redirect_uri")
	clientID := r.URL.Query().Get("client_id")
	if redirect == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	client, err := h.registry.Lookup(clientID)
	if err != nil || !clientAllowsRedirect(client, redirect) {
		// An unregistered post-logout redirect is refused (open-redirect risk).
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if state := r.URL.Query().Get("state"); state != "" {
		if sep := "?"; redirect != "" {
			if hasQuery(redirect) {
				sep = "&"
			}
			redirect = redirect + sep + "state=" + state
		}
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func hasQuery(u string) bool {
	for i := 0; i < len(u); i++ {
		if u[i] == '?' {
			return true
		}
	}
	return false
}
