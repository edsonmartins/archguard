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
	"net/url"
	"strings"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// AuthCodeIssuer mints and stores an authorization code for an authenticated
// session — the postgres adapter implements it. It returns the code SECRET to put
// in the redirect.
type AuthCodeIssuer interface {
	IssueCode(ctx context.Context, clientID, redirectURI, codeChallenge string, session *domain.AuthSession, scopes []string) (string, error)
}

// AuthorizeHandler serves GET /authorize — the Authorization Code endpoint
// (RFC-0006 §2). It validates the request against the flow policy and the client
// registry (only "code", S256 PKCE mandatory, registered client/redirect), then —
// given an already authenticated session (resolved via SessionResolver, the
// login seam; the login UI itself is pacote 008) — issues a code and redirects.
// It is fail-closed: any validation failure refuses BEFORE a code exists.
type AuthorizeHandler struct {
	registry *domain.ClientRegistry
	resolve  SessionResolver
	issuer   AuthCodeIssuer
	now      func() time.Time
}

// NewAuthorizeHandler builds the handler.
func NewAuthorizeHandler(registry *domain.ClientRegistry, resolve SessionResolver, issuer AuthCodeIssuer) *AuthorizeHandler {
	return &AuthorizeHandler{registry: registry, resolve: resolve, issuer: issuer, now: time.Now}
}

func (h *AuthorizeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	state := q.Get("state")
	scopes := strings.Fields(q.Get("scope"))

	// The client must be registered and allowed the Authorization Code flow.
	client, err := h.registry.AuthorizeClientFlow(clientID, domain.FlowAuthorizationCode)
	if err != nil {
		// An unknown client / bad redirect must NOT be redirected to (open-redirect
		// risk): answer directly.
		writeError(w, http.StatusBadRequest, "client_id inválido")
		return
	}
	if !clientAllowsRedirect(client, redirectURI) {
		writeError(w, http.StatusBadRequest, "redirect_uri não registrado")
		return
	}

	// From here a validation failure can be reported to the client via the
	// redirect (RFC 6749 §4.1.2.1) — the redirect_uri is now trusted.
	if err := domain.ValidateAuthorizationCodeRequest(responseType, codeChallenge, codeChallengeMethod); err != nil {
		redirectError(w, r, redirectURI, "invalid_request", err.Error(), state)
		return
	}

	// The user must already be authenticated (login seam). Without a session the
	// real flow redirects to the login UI (pacote 008); here we signal login
	// required rather than inventing a UI.
	session, ok := h.resolve.Session(r)
	if !ok || session == nil {
		redirectError(w, r, redirectURI, "login_required", "autenticação necessária", state)
		return
	}
	// Minimal scope: only the intersection of requested and the client's allowed.
	granted := domain.MinimalScope(scopes, client.AllowedScopes)

	secret, err := h.issuer.IssueCode(r.Context(), clientID, redirectURI, codeChallenge, session, granted)
	if err != nil {
		redirectError(w, r, redirectURI, "server_error", "não foi possível emitir o código", state)
		return
	}
	redirectSuccess(w, r, redirectURI, secret, state)
}

func clientAllowsRedirect(c domain.OIDCClient, redirectURI string) bool {
	for _, u := range c.RedirectURIs {
		if u == redirectURI {
			return true
		}
	}
	return false
}

func redirectSuccess(w http.ResponseWriter, r *http.Request, redirectURI, code, state string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, "redirect_uri inválido")
		return
	}
	qq := u.Query()
	qq.Set("code", code)
	if state != "" {
		qq.Set("state", state)
	}
	u.RawQuery = qq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, code, desc, state string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, "redirect_uri inválido")
		return
	}
	qq := u.Query()
	qq.Set("error", code)
	qq.Set("error_description", desc)
	if state != "" {
		qq.Set("state", state)
	}
	u.RawQuery = qq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}
