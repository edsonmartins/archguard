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
	"fmt"
	"net/http"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// SessionResolver extracts the authenticated session from a request — placed
// there by the authentication layer earlier in the chain. It returns false when
// there is no session, which the middleware treats as no proven assurance (deny).
type SessionResolver interface {
	Session(r *http.Request) (*domain.AuthSession, bool)
}

// TenantFloor answers the active tenant's MFA floor for a session, so the
// middleware can apply "a mais restritiva vence" (T-011). It is
// domain.TenantAuthPolicy narrowed to what the middleware needs. Fail-closed: an
// implementation that cannot decide returns an error, and the middleware denies.
type TenantFloor interface {
	RequiredAAL(ctx context.Context, organizationID uuid.UUID) (domain.AAL, error)
}

// AssuranceMiddleware enforces operation classification (INV-8 / ADR-0010) on
// HTTP handlers. It is thin (CLAUDE.md §6): it resolves the session, asks the
// domain guard, and on denial emits a step-up CHALLENGE that names the acr the
// client must reach — it holds no policy of its own.
type AssuranceMiddleware struct {
	guard   *domain.AssuranceGuard
	resolve SessionResolver
	floor   TenantFloor
	// now supplies the current instant for the freshness check; overridable in
	// tests. Defaults to time.Now.
	now func() time.Time
}

// NewAssuranceMiddleware builds the middleware over the guard, a session resolver
// and the tenant-floor policy (the active tenant's MFA minimum, composed by the
// guard as "the more restrictive wins").
func NewAssuranceMiddleware(guard *domain.AssuranceGuard, resolve SessionResolver, floor TenantFloor) *AssuranceMiddleware {
	return &AssuranceMiddleware{guard: guard, resolve: resolve, floor: floor, now: time.Now}
}

// Require wraps next so it runs ONLY if the request's session satisfies
// operationID's classified level. On an insufficient-assurance denial it answers
// 401 with an RFC 9470 step-up challenge (acr_values naming the required acr) so
// the client knows exactly what to re-authenticate with. An unclassified
// operation is a server-side defect (a route with no level, which T-017's build
// gate exists to prevent) and answers 500 — still fail-closed, next never runs.
func (m *AssuranceMiddleware) Require(operationID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := m.resolve.Session(r)

		// Resolve the active tenant's MFA floor so the guard applies "the more
		// restrictive wins". A session with no active tenant has no floor to add
		// (AAL1); a floor lookup that FAILS is fail-closed (deny), never ignored.
		floor := domain.AAL1
		if session != nil {
			if _, orgID, err := session.ActiveTenant(); err == nil {
				aal, ferr := m.floor.RequiredAAL(r.Context(), orgID)
				if ferr != nil {
					writeAssuranceError(w, http.StatusInternalServerError, "policy_unavailable",
						"política de MFA do tenant indisponível", operationID, nil)
					return
				}
				floor = aal
			}
		}

		err := m.guard.Authorize(operationID, session, floor, m.now())
		if err == nil {
			next.ServeHTTP(w, r)
			return
		}

		var iae *domain.InsufficientAssuranceError
		switch {
		case errors.As(err, &iae):
			writeStepUpChallenge(w, iae)
		case errors.Is(err, domain.ErrOperationNotClassified):
			// The route reached the middleware without a classification — a wiring
			// bug. Deny (never run next) and answer a server error, leaking nothing.
			writeAssuranceError(w, http.StatusInternalServerError, "operation_not_classified",
				"operação sem classificação de garantia", "", nil)
		default:
			writeAssuranceError(w, http.StatusForbidden, "assurance_denied", "acesso negado", "", nil)
		}
	})
}

// assuranceErrorBody is the JSON denial body — machine-readable so the console
// can drive the step-up flow.
type assuranceErrorBody struct {
	Error                  string `json:"error"`
	ErrorDescription       string `json:"error_description"`
	RequiredLevel          string `json:"required_level,omitempty"`
	ACRValues              string `json:"acr_values,omitempty"`
	NeedsPhishingResistant *bool  `json:"needs_phishing_resistant,omitempty"`
	Operation              string `json:"operation,omitempty"`
}

// writeStepUpChallenge answers an insufficient-assurance denial with RFC 9470's
// step-up challenge: 401, a WWW-Authenticate header carrying acr_values, and a
// JSON body repeating the requirement.
func writeStepUpChallenge(w http.ResponseWriter, iae *domain.InsufficientAssuranceError) {
	// RFC 9470 / RFC 6750: Bearer challenge with error and acr_values.
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(
		`Bearer error="insufficient_user_authentication", error_description=%q, acr_values=%q`,
		"garantia insuficiente para a operação", iae.RequiredACR))
	needs := iae.NeedsPhishingResistant
	writeAssuranceError(w, http.StatusUnauthorized, "insufficient_user_authentication",
		"garantia insuficiente para a operação", iae.Operation, &assuranceErrorBody{
			RequiredLevel:          string(iae.Required),
			ACRValues:              iae.RequiredACR,
			NeedsPhishingResistant: &needs,
		})
}

// writeAssuranceError writes a denial body with the given status. extra carries
// the challenge-specific fields (required level, acr_values); it may be nil.
func writeAssuranceError(w http.ResponseWriter, status int, code, description, operation string, extra *assuranceErrorBody) {
	body := assuranceErrorBody{Error: code, ErrorDescription: description, Operation: operation}
	if extra != nil {
		body.RequiredLevel = extra.RequiredLevel
		body.ACRValues = extra.ACRValues
		body.NeedsPhishingResistant = extra.NeedsPhishingResistant
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
