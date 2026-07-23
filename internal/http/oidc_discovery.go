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

	"github.com/casdoor/casdoor/internal/domain"
)

// DiscoveryHandler serves GET /.well-known/openid-configuration — the OpenID
// Connect discovery metadata (pacote 006 wiring). It is thin: it holds the
// pre-built document and encodes it. The document is built from the flow policy,
// so it never advertises a flow the server refuses.
type DiscoveryHandler struct {
	doc domain.DiscoveryDocument
}

// NewDiscoveryHandler builds the handler over a discovery document.
func NewDiscoveryHandler(doc domain.DiscoveryDocument) *DiscoveryHandler {
	return &DiscoveryHandler{doc: doc}
}

func (h *DiscoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	writeJSON(w, http.StatusOK, h.doc)
}

// JWKSSource provides the current published JSON Web Key Set — implemented by the
// oidc.Signer (its JWKS includes the current and retained previous keys).
type JWKSSource interface {
	JWKS() ([]byte, error)
}

// JWKSHandler serves GET /jwks — the public keys components validate tokens with.
type JWKSHandler struct {
	source JWKSSource
}

// NewJWKSHandler builds the handler over a JWKS source.
func NewJWKSHandler(source JWKSSource) *JWKSHandler {
	return &JWKSHandler{source: source}
}

func (h *JWKSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	jwks, err := h.source.JWKS()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "JWKS indisponível")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jwks)
}
