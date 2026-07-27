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
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeBreakglassRequester struct {
	err         error
	gotActor    RevokeActor
	gotOrg      uuid.UUID
	gotTarget   domain.GrantTarget
	gotJustif   string
	gotIncident string
}

func (f *fakeBreakglassRequester) RequestBreakglass(_ context.Context, actor RevokeActor, _ domain.AAL, _ bool, org uuid.UUID, target domain.GrantTarget, justification, incidentRef string, _, _ time.Time) error {
	f.gotActor, f.gotOrg, f.gotTarget = actor, org, target
	f.gotJustif, f.gotIncident = justification, incidentRef
	return f.err
}

// sessionWithMembership builds an active session with both an active tenant and a
// membership (the subject that would receive the break-glass access).
func sessionWithMembership(orgID uuid.UUID) *domain.AuthSession {
	membership := uuid.New()
	return &domain.AuthSession{IdentityID: uuid.New(), OrganizationID: &orgID, MembershipID: &membership, ID: uuid.New(), Status: domain.SessionActive}
}

func unix(d time.Duration) string {
	return strconv.FormatInt(time.Now().Add(d).Unix(), 10)
}

func futureBody() string {
	return `{"target_type":"database","target_id":"prod-oracle-01","target_scope":"read","justification":"incidente em prod","incident_ref":"INC-42","expires_at":` + unix(2*time.Hour) + `}`
}

func bgRequest(session *domain.AuthSession, bodyJSON string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/breakglass/request", strings.NewReader(bodyJSON))
	if session != nil {
		req = req.WithContext(withSession(req.Context(), session))
	}
	return req
}

func TestBreakglassRequestSucceeds(t *testing.T) {
	orgID := uuid.New()
	session := sessionWithMembership(orgID)
	requester := &fakeBreakglassRequester{}
	rr := httptest.NewRecorder()
	NewBreakglassRequestHandler(requester).ServeHTTP(rr, bgRequest(session, futureBody()))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201 — body %s", rr.Code, rr.Body.String())
	}
	// Actor and tenant come from the SESSION (INV-1/INV-5); the subject is the session's
	// own membership; target/justification/incident from the body.
	if requester.gotActor.IdentityID != session.IdentityID || requester.gotActor.MembershipID != session.MembershipID {
		t.Fatalf("actor not from session: %+v", requester.gotActor)
	}
	if requester.gotOrg != orgID {
		t.Fatalf("org %v, want %v", requester.gotOrg, orgID)
	}
	if requester.gotTarget.Type != "database" || requester.gotTarget.ID != "prod-oracle-01" || requester.gotTarget.Scope != "read" {
		t.Fatalf("target mismatch: %+v", requester.gotTarget)
	}
	if requester.gotJustif == "" || requester.gotIncident != "INC-42" {
		t.Fatalf("justification/incident not forwarded: %q / %q", requester.gotJustif, requester.gotIncident)
	}
}

func TestBreakglassChannelUnavailableIs503(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	rr := httptest.NewRecorder()
	NewBreakglassRequestHandler(&fakeBreakglassRequester{err: ErrBreakglassChannelUnavailable}).
		ServeHTTP(rr, bgRequest(session, futureBody()))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("channel unavailable: status %d, want 503", rr.Code)
	}
}

func TestBreakglassNeedsWebAuthnIs403(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	rr := httptest.NewRecorder()
	NewBreakglassRequestHandler(&fakeBreakglassRequester{err: ErrBreakglassNeedsWebAuthn}).
		ServeHTTP(rr, bgRequest(session, futureBody()))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("needs webauthn: status %d, want 403", rr.Code)
	}
}

func TestBreakglassInvalidIs422(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	rr := httptest.NewRecorder()
	NewBreakglassRequestHandler(&fakeBreakglassRequester{err: ErrBreakglassInvalid}).
		ServeHTTP(rr, bgRequest(session, futureBody()))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: status %d, want 422", rr.Code)
	}
}

func TestBreakglassRequestValidation(t *testing.T) {
	orgID := uuid.New()
	full := sessionWithMembership(orgID)
	noMembership := &domain.AuthSession{IdentityID: uuid.New(), OrganizationID: &orgID, ID: uuid.New(), Status: domain.SessionActive}
	noOrg := &domain.AuthSession{IdentityID: uuid.New(), ID: uuid.New(), Status: domain.SessionPendingSelection}

	cases := []struct {
		name    string
		method  string
		session *domain.AuthSession
		body    string
		want    int
	}{
		{"wrong method", http.MethodGet, full, futureBody(), http.StatusMethodNotAllowed},
		{"no session", http.MethodPost, nil, futureBody(), http.StatusUnauthorized},
		{"no active tenant", http.MethodPost, noOrg, futureBody(), http.StatusConflict},
		{"no membership", http.MethodPost, noMembership, futureBody(), http.StatusConflict},
		{"missing justification", http.MethodPost, full, `{"target_type":"db","target_id":"x","incident_ref":"INC-1","expires_at":` + unix(time.Hour) + `}`, http.StatusBadRequest},
		{"missing target", http.MethodPost, full, `{"justification":"j","incident_ref":"INC-1","expires_at":` + unix(time.Hour) + `}`, http.StatusBadRequest},
		{"missing expires_at", http.MethodPost, full, `{"target_type":"db","target_id":"x","justification":"j","incident_ref":"INC-1"}`, http.StatusBadRequest},
		{"past expires_at", http.MethodPost, full, `{"target_type":"db","target_id":"x","justification":"j","incident_ref":"INC-1","expires_at":` + unix(-time.Hour) + `}`, http.StatusBadRequest},
		{"bad body", http.MethodPost, full, `{`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/breakglass/request", strings.NewReader(tc.body))
			if tc.session != nil {
				req = req.WithContext(withSession(req.Context(), tc.session))
			}
			rr := httptest.NewRecorder()
			NewBreakglassRequestHandler(&fakeBreakglassRequester{}).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status %d, want %d — body %s", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}
