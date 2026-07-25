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

import "net/http"

// SessionContextHandler serves GET /session: the caller's own session context —
// identity, active tenant and proven assurance. It is the first thing the console
// loads (who am I, which tenant), and reads the session the assurance middleware
// injected; it holds no logic of its own (CLAUDE.md §6).
type SessionContextHandler struct{}

// NewSessionContextHandler builds the handler.
func NewSessionContextHandler() *SessionContextHandler { return &SessionContextHandler{} }

// sessionContextResponse is the JSON body. Personal data (e-mail, name) is NOT
// included — the session context carries identifiers and assurance only (I-3.2).
type sessionContextResponse struct {
	IdentityID     string   `json:"identity_id"`
	Status         string   `json:"status"`
	OrganizationID string   `json:"organization_id,omitempty"`
	MembershipID   string   `json:"membership_id,omitempty"`
	ProvenAAL      string   `json:"proven_aal"`
	AuthTime       int64    `json:"auth_time,omitempty"`
	AMR            []string `json:"amr,omitempty"`
}

func (h *SessionContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		// The middleware authorizes before dispatching, so this is defensive: no
		// session in context is fail-closed, never an empty context served as 200.
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}

	resp := sessionContextResponse{
		IdentityID: session.IdentityID.String(),
		Status:     string(session.Status),
		ProvenAAL:  string(session.ProvenAAL),
	}
	if session.OrganizationID != nil {
		resp.OrganizationID = session.OrganizationID.String()
	}
	if session.MembershipID != nil {
		resp.MembershipID = session.MembershipID.String()
	}
	if !session.AuthTime.IsZero() {
		resp.AuthTime = session.AuthTime.Unix()
	}
	for _, m := range session.AuthMethods {
		resp.AMR = append(resp.AMR, string(m))
	}
	writeJSON(w, http.StatusOK, resp)
}
