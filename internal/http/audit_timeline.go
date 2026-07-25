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
	"strconv"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

const (
	auditTimelineDefaultLimit = 50
	auditTimelineMaxLimit     = 200
)

// AuditReader lists a tenant's recent audit events
// (postgres.TenantAuditReader implements it).
type AuditReader interface {
	ListRecent(ctx context.Context, organizationID uuid.UUID, limit int) ([]domain.SealedEvent, error)
}

// AuditTimelineHandler serves GET /audit/timeline?limit=N: the caller's ACTIVE
// tenant's recent audit events, newest first — the console's auditing view. An
// administration read (RequireAdmin gates it). It exposes only non-personal fields
// (the actor is the opaque subject, never plaintext identity — I-3.2) and never the
// chain hashes. Thin (§6).
type AuditTimelineHandler struct {
	reader AuditReader
}

// NewAuditTimelineHandler builds the handler over an audit reader.
func NewAuditTimelineHandler(reader AuditReader) *AuditTimelineHandler {
	return &AuditTimelineHandler{reader: reader}
}

// auditEventItem is one displayed event — identifiers, verb, outcome and
// correlation only; no chain hashes, no origin IP/user-agent.
type auditEventItem struct {
	Seq          int64  `json:"seq"`
	OccurredAt   int64  `json:"occurred_at"`
	Action       string `json:"action"`
	Outcome      string `json:"outcome"`
	ActorSubject string `json:"actor_subject"`
	TargetType   string `json:"target_type,omitempty"`
	TargetID     string `json:"target_id,omitempty"`
	TargetLabel  string `json:"target_label,omitempty"`
	Reason       string `json:"reason,omitempty"`
	PCID         string `json:"pcid,omitempty"`
}

func (h *AuditTimelineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "sessão não resolvida")
		return
	}
	if session.OrganizationID == nil {
		writeError(w, http.StatusConflict, "nenhum tenant ativo na sessão")
		return
	}

	events, err := h.reader.ListRecent(r.Context(), *session.OrganizationID, auditLimit(r))
	if err != nil {
		// A trail read that could not run is fail-closed: never an empty timeline
		// served as authoritative.
		writeError(w, http.StatusInternalServerError, "não foi possível ler a trilha")
		return
	}

	items := make([]auditEventItem, 0, len(events))
	for _, se := range events {
		items = append(items, auditEventItem{
			Seq:          se.Seq,
			OccurredAt:   se.Event.OccurredAt.Unix(),
			Action:       string(se.Event.Action),
			Outcome:      se.Event.SerializedOutcome(),
			ActorSubject: se.Event.Actor.IdentitySubject,
			TargetType:   se.Event.Target.Type,
			TargetID:     se.Event.Target.ID,
			TargetLabel:  se.Event.Target.Label,
			Reason:       se.Event.Reason,
			PCID:         se.Event.Context.PrivilegedCorrelationID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": items})
}

// auditLimit reads and clamps the page size to [1, auditTimelineMaxLimit],
// defaulting when absent or malformed.
func auditLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return auditTimelineDefaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return auditTimelineDefaultLimit
	}
	if n > auditTimelineMaxLimit {
		return auditTimelineMaxLimit
	}
	return n
}
