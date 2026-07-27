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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ErrSelfApproval lets the handler answer 403 when the approver is the grant's own
// requester — separation of duties (boot maps the domain sentinel).
var ErrSelfApproval = errors.New("o solicitante não pode aprovar a própria solicitação")

// ErrDuplicateApproval lets the handler answer 409 when the same approver approves twice
// (boot maps the domain sentinel).
var ErrDuplicateApproval = errors.New("aprovação duplicada do mesmo par")

// BreakglassPendingLister lists the tenant's break-glass grants awaiting peer approval
// (postgres.TenantGrantLister implements it).
type BreakglassPendingLister interface {
	ListAwaitingApproval(ctx context.Context, organizationID uuid.UUID) ([]domain.PrivilegedGrant, error)
}

// pendingBreakglassItem is one grant awaiting approval. It DOES carry the justification and
// incident (unlike the external alert, which omits the justification) — the approver is an
// administrator who needs them to decide.
type pendingBreakglassItem struct {
	GrantID             string `json:"grant_id"`
	SubjectMembershipID string `json:"subject_membership_id"`
	TargetType          string `json:"target_type"`
	TargetID            string `json:"target_id"`
	TargetScope         string `json:"target_scope,omitempty"`
	Justification       string `json:"justification"`
	IncidentRef         string `json:"incident_ref"`
	RequiredApprovals   int    `json:"required_approvals"`
	NotBefore           int64  `json:"not_before"`
	ExpiresAt           int64  `json:"expires_at"`
}

// BreakglassPendingHandler serves GET /breakglass/pending: the break-glass grants of the
// caller's ACTIVE tenant awaiting approval. Administration view (RequireAdmin at the mount,
// L1 read). Reads the active organization from the session, never the request. Thin (§6).
type BreakglassPendingHandler struct {
	lister BreakglassPendingLister
}

// NewBreakglassPendingHandler builds the handler over a pending lister.
func NewBreakglassPendingHandler(lister BreakglassPendingLister) *BreakglassPendingHandler {
	return &BreakglassPendingHandler{lister: lister}
}

func (h *BreakglassPendingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	grants, err := h.lister.ListAwaitingApproval(r.Context(), *session.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar as solicitações pendentes")
		return
	}
	items := make([]pendingBreakglassItem, 0, len(grants))
	for _, g := range grants {
		items = append(items, pendingBreakglassItem{
			GrantID:             g.ID.String(),
			SubjectMembershipID: g.SubjectMembershipID.String(),
			TargetType:          g.Target.Type,
			TargetID:            g.Target.ID,
			TargetScope:         g.Target.Scope,
			Justification:       g.Justification,
			IncidentRef:         g.IncidentRef,
			RequiredApprovals:   g.RequiredApprovals,
			NotBefore:           g.NotBefore.Unix(),
			ExpiresAt:           g.ExpiresAt.Unix(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending": items})
}

// BreakglassApprover records one peer approval on a break-glass grant (boot composes it
// over postgres.PrivilegedAccessService + the audit writer).
type BreakglassApprover interface {
	ApproveBreakglass(ctx context.Context, actor RevokeActor, organizationID, grantID uuid.UUID) error
}

// BreakglassApproveHandler serves POST /breakglass/approve {"grant_id":"…"}: it records the
// CALLER's approval on a grant of their active tenant. The domain enforces separation of
// duties (the requester cannot approve — ErrSelfApproval; distinct approvers — no duplicate)
// and activates the grant once the quorum is met, all atomically with the audit. L3
// administration write (step-up + admin gate at the mount). The approver membership and the
// organization come from the session, never the request (INV-1). Thin (§6).
type BreakglassApproveHandler struct {
	approver BreakglassApprover
}

// NewBreakglassApproveHandler builds the handler over an approver.
func NewBreakglassApproveHandler(approver BreakglassApprover) *BreakglassApproveHandler {
	return &BreakglassApproveHandler{approver: approver}
}

func (h *BreakglassApproveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	if session.MembershipID == nil {
		writeError(w, http.StatusConflict, "sessão sem membership ativo para aprovar")
		return
	}
	var body struct {
		GrantID string `json:"grant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.GrantID == "" {
		writeError(w, http.StatusBadRequest, "grant_id obrigatório")
		return
	}
	grantID, err := uuid.Parse(body.GrantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "grant_id inválido")
		return
	}

	actor := RevokeActor{
		IdentityID:   session.IdentityID,
		MembershipID: session.MembershipID,
		SessionID:    session.ID,
	}
	err = h.approver.ApproveBreakglass(r.Context(), actor, *session.OrganizationID, grantID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"approved": true})
	case errors.Is(err, ErrSelfApproval):
		writeError(w, http.StatusForbidden, "o solicitante não pode aprovar a própria solicitação")
	case errors.Is(err, ErrDuplicateApproval):
		writeError(w, http.StatusConflict, "você já aprovou esta solicitação")
	case errors.Is(err, ErrGrantNotActive):
		writeError(w, http.StatusConflict, "a solicitação não está aguardando aprovação")
	case errors.Is(err, ErrGrantNotFound):
		writeError(w, http.StatusNotFound, "solicitação não encontrada neste tenant")
	default:
		// Any other failure (including a failed audit write) is fail-closed: the approval
		// did not commit (I-5.4).
		writeError(w, http.StatusInternalServerError, "não foi possível aprovar a solicitação")
	}
}
