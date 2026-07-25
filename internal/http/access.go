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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// AccessDecider answers an authorization question (postgres.PostgresPDP implements
// it). The handler depends on this narrow port, not on the adapter.
type AccessDecider interface {
	Check(ctx context.Context, req domain.CheckRequest) (domain.Decision, error)
}

// AccessHandler serves GET /access/effective?membership=<uuid>&asset=<id>: the
// effective privileged access of a membership over an asset, as the PDP decides it
// (RFC-0004) — the console's access-review view (008 T-020). It reports both
// can_open_session (structural: operator/owner) and can_open_privileged_session
// (structural AND an active grant). An administration view (RequireAdmin gates it).
//
// A PDP that cannot answer (ErrPDPUnavailable) is reported as 503 — the review
// could not run. It is NEVER shown as "no access", which would misread as a
// definitive denial (fail-closed distinguishes "denied" from "could not decide").
type AccessHandler struct {
	decider AccessDecider
	now     func() time.Time
}

// NewAccessHandler builds the handler over an access decider.
func NewAccessHandler(decider AccessDecider) *AccessHandler {
	return &AccessHandler{decider: decider, now: time.Now}
}

type accessResponse struct {
	MembershipID             string `json:"membership_id"`
	AssetID                  string `json:"asset_id"`
	CanOpenSession           bool   `json:"can_open_session"`
	CanOpenPrivilegedSession bool   `json:"can_open_privileged_session"`
}

func (h *AccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	membership := r.URL.Query().Get("membership")
	asset := r.URL.Query().Get("asset")
	if membership == "" || asset == "" {
		writeError(w, http.StatusBadRequest, "membership e asset são obrigatórios")
		return
	}

	orgID := *session.OrganizationID
	membershipRef := domain.Qualify(orgID, domain.TypeMembership, membership)
	assetRef := domain.Qualify(orgID, domain.TypeAsset, asset)
	cc := domain.CheckContext{ACR: session.ACR(), EvaluatedAt: h.now(), Origin: "console"}

	canOpen, err := h.decideRelation(r.Context(), membershipRef, domain.RelCanOpenSession, assetRef, cc)
	if err != nil {
		writeAccessError(w, err)
		return
	}
	canPriv, err := h.decideRelation(r.Context(), membershipRef, domain.RelCanOpenPrivilegedSession, assetRef, cc)
	if err != nil {
		writeAccessError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, accessResponse{
		MembershipID:             membership,
		AssetID:                  asset,
		CanOpenSession:           canOpen,
		CanOpenPrivilegedSession: canPriv,
	})
}

func (h *AccessHandler) decideRelation(ctx context.Context, user, relation, object string, cc domain.CheckContext) (bool, error) {
	dec, err := h.decider.Check(ctx, domain.CheckRequest{
		Tuple:   domain.RelationTuple{User: user, Relation: relation, Object: object},
		Context: cc,
	})
	if err != nil {
		return false, err
	}
	return dec.Allowed, nil
}

// writeAccessError distinguishes "the PDP could not decide" (503, fail-closed —
// never shown as a definitive denial) from a malformed request (400).
func writeAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrPDPUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "PDP indisponível — a revisão não pôde ser concluída")
		return
	}
	writeError(w, http.StatusBadRequest, "requisição de decisão inválida")
}
