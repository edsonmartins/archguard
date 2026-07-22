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

// Package http holds thin HTTP handlers that translate HTTP ↔ domain and
// nothing more (CLAUDE.md §6): no business logic lives here. The audit
// verification handler is the first — it exposes the T-013 verifier as an
// endpoint.
package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// AuditVerifyAssuranceLevel is the assurance level this operation declares
// (INV-8 / ADR-0010): verifying the audit trail is a privileged operation, so
// it is L3 — the routing layer MUST require an L3-authenticated caller
// (immediate WebAuthn re-auth) before dispatching here. Declaring it in code
// keeps the contract next to the handler.
const AuditVerifyAssuranceLevel = domain.L3

// OrganizationVerifier verifies one organization's trail (implemented by
// postgres.AuditVerifier). The handler depends on this narrow port, not on the
// adapter.
type OrganizationVerifier interface {
	VerifyOrganization(ctx context.Context, organizationID uuid.UUID) (domain.VerifyReport, error)
}

// AuditVerifyHandler serves GET /audit/verify?organization_id=<uuid>: it runs
// the trail verification for the organization and returns the report as JSON.
// A divergence is reported with HTTP 409 (Conflict) so a monitor can alert on
// the status alone; an intact trail returns 200. The handler is thin — it
// parses the request, calls the verifier, and encodes the report.
type AuditVerifyHandler struct {
	verifier OrganizationVerifier
}

// NewAuditVerifyHandler builds the handler over a verifier.
func NewAuditVerifyHandler(verifier OrganizationVerifier) *AuditVerifyHandler {
	return &AuditVerifyHandler{verifier: verifier}
}

// verifyResponse is the JSON body — the report plus a human-readable status.
type verifyResponse struct {
	OrganizationID        string `json:"organization_id"`
	OK                    bool   `json:"ok"`
	EventsChecked         int    `json:"events_checked"`
	SealsChecked          int    `json:"seals_checked"`
	SealSignaturesChecked bool   `json:"seal_signatures_checked"`
	FirstDivergence       int64  `json:"first_divergence_seq,omitempty"`
	Kind                  string `json:"divergence_kind,omitempty"`
	Detail                string `json:"detail,omitempty"`
}

func (h *AuditVerifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	orgParam := r.URL.Query().Get("organization_id")
	orgID, err := uuid.Parse(orgParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "organization_id inválido")
		return
	}

	rep, err := h.verifier.VerifyOrganization(r.Context(), orgID)
	if err != nil {
		// A verification that could not run is fail-closed: report it as a
		// server error, never as "intact".
		writeError(w, http.StatusInternalServerError, "verificação não pôde ser concluída")
		return
	}

	resp := verifyResponse{
		OrganizationID:        orgID.String(),
		OK:                    rep.OK,
		EventsChecked:         rep.EventsChecked,
		SealsChecked:          rep.SealsChecked,
		SealSignaturesChecked: rep.SealSignaturesChecked,
		FirstDivergence:       rep.FirstDivergence,
		Kind:                  string(rep.Kind),
		Detail:                rep.Detail,
	}
	status := http.StatusOK
	if !rep.OK {
		status = http.StatusConflict // divergence — the trail was tampered with
	}
	writeJSON(w, status, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
