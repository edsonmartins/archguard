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
	"net/http"

	"github.com/casdoor/casdoor/internal/domain"
)

// RefreshGrant performs a refresh-token exchange: it rotates the presented
// refresh secret and mints a new access token, or reports reuse. The postgres
// adapter implements it (composing ResolveRefreshTokenOrg + RefreshExchanger +
// TokenIssuer). It is the seam the token endpoint depends on, so the handler
// stays thin.
type RefreshGrant interface {
	// Refresh exchanges presentedSecret for a new access token and refresh secret.
	// It returns domain.ErrRefreshReuse on reuse (the family was revoked) and a
	// not-found error for an unknown token.
	Refresh(ctx context.Context, presentedSecret string) (domain.RefreshResult, error)
}

// tokenErrorBody is the OAuth 2.0 token error response (RFC 6749 §5.2).
type tokenErrorBody struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

// tokenSuccessBody is the OAuth 2.0 token success response.
type tokenSuccessBody struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// TokenHandler serves POST /token. It supports the refresh_token grant here
// (authorization_code is added with the authorization endpoint). Implicit and
// ROPC are refused by ValidateGrantType. A reuse-detected refresh returns
// invalid_grant AFTER the family has been revoked (the security response, not a
// mere error).
type TokenHandler struct {
	refresh RefreshGrant
}

// NewTokenHandler builds the handler over a refresh grant.
func NewTokenHandler(refresh RefreshGrant) *TokenHandler {
	return &TokenHandler{refresh: refresh}
}

func (h *TokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "form inválido")
		return
	}
	grantType := r.PostForm.Get("grant_type")
	if err := domain.ValidateGrantType(grantType); err != nil {
		// Refuses ROPC and unsupported grants (T-005).
		writeTokenError(w, http.StatusBadRequest, "unsupported_grant_type", err.Error())
		return
	}

	switch grantType {
	case "refresh_token":
		h.handleRefresh(w, r)
	default:
		// authorization_code / device_code chegam com a fiação do /authorize.
		writeTokenError(w, http.StatusBadRequest, "unsupported_grant_type", "grant ainda não fiado neste endpoint")
	}
}

func (h *TokenHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	secret := r.PostForm.Get("refresh_token")
	if secret == "" {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "refresh_token ausente")
		return
	}
	res, err := h.refresh.Refresh(r.Context(), secret)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshReuse) {
			// Reuse detected: the family was revoked. RFC 6749: invalid_grant.
			writeTokenError(w, http.StatusBadRequest, "invalid_grant", "reuso de refresh token detectado — sessão encerrada")
			return
		}
		// Unknown/expired token — a bare invalid_grant, no detail leaked.
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "refresh token inválido")
		return
	}
	// A rotated refresh token MUST NOT be cached by intermediaries.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, tokenSuccessBody{
		AccessToken:  res.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    res.ExpiresInSecond,
		RefreshToken: res.RefreshToken,
	})
}

func writeTokenError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, tokenErrorBody{Error: code, Description: desc})
}
