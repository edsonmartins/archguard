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
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakePendingLister struct {
	grants []domain.PrivilegedGrant
	err    error
	gotOrg uuid.UUID
}

func (f *fakePendingLister) ListAwaitingApproval(_ context.Context, org uuid.UUID) ([]domain.PrivilegedGrant, error) {
	f.gotOrg = org
	return f.grants, f.err
}

type fakeApprover struct {
	err      error
	gotActor RevokeActor
	gotOrg   uuid.UUID
	gotGrant uuid.UUID
}

func (f *fakeApprover) ApproveBreakglass(_ context.Context, actor RevokeActor, org, grant uuid.UUID) error {
	f.gotActor, f.gotOrg, f.gotGrant = actor, org, grant
	return f.err
}

func TestBreakglassPendingLists(t *testing.T) {
	orgID := uuid.New()
	session := sessionWithMembership(orgID)
	lister := &fakePendingLister{grants: []domain.PrivilegedGrant{{
		ID:                  uuid.New(),
		SubjectMembershipID: uuid.New(),
		Target:              domain.GrantTarget{Type: "database", ID: "prod-01", Scope: "admin"},
		Justification:       "prod fora do ar",
		IncidentRef:         "INC-9",
		RequiredApprovals:   2,
	}}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/breakglass/pending", nil).WithContext(withSession(context.Background(), session))
	NewBreakglassPendingHandler(lister).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	if lister.gotOrg != orgID {
		t.Fatalf("listed org %v, want session org %v", lister.gotOrg, orgID)
	}
	// The queue MUST expose justification and incident — the approver decides with them.
	body := rr.Body.String()
	if !strings.Contains(body, "prod fora do ar") || !strings.Contains(body, "INC-9") {
		t.Fatalf("pending item should carry justification and incident: %s", body)
	}
}

func TestBreakglassApproveSucceeds(t *testing.T) {
	orgID := uuid.New()
	grant := uuid.New()
	session := sessionWithMembership(orgID)
	approver := &fakeApprover{}
	rr := httptest.NewRecorder()
	NewBreakglassApproveHandler(approver).ServeHTTP(rr, bgRequest(session, `{"grant_id":"`+grant.String()+`"}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 — body %s", rr.Code, rr.Body.String())
	}
	// The approver is the caller's own session membership (INV-1); org and grant match.
	if approver.gotActor.MembershipID != session.MembershipID || approver.gotOrg != orgID || approver.gotGrant != grant {
		t.Fatalf("approve used %+v / %v / %v", approver.gotActor, approver.gotOrg, approver.gotGrant)
	}
	if !strings.Contains(rr.Body.String(), `"approved":true`) {
		t.Fatalf("body should confirm approval: %s", rr.Body.String())
	}
}

func TestBreakglassApproveSeparationOfDuties(t *testing.T) {
	orgID := uuid.New()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"self approval", ErrSelfApproval, http.StatusForbidden},
		{"duplicate", ErrDuplicateApproval, http.StatusConflict},
		{"not awaiting", ErrGrantNotActive, http.StatusConflict},
		{"not found", ErrGrantNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := sessionWithMembership(orgID)
			rr := httptest.NewRecorder()
			NewBreakglassApproveHandler(&fakeApprover{err: tc.err}).
				ServeHTTP(rr, bgRequest(session, `{"grant_id":"`+uuid.NewString()+`"}`))
			if rr.Code != tc.want {
				t.Fatalf("%s: status %d, want %d", tc.name, rr.Code, tc.want)
			}
		})
	}
}

func TestBreakglassApproveValidation(t *testing.T) {
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
		{"wrong method", http.MethodGet, full, `{"grant_id":"` + uuid.NewString() + `"}`, http.StatusMethodNotAllowed},
		{"no session", http.MethodPost, nil, `{"grant_id":"x"}`, http.StatusUnauthorized},
		{"no active tenant", http.MethodPost, noOrg, `{"grant_id":"` + uuid.NewString() + `"}`, http.StatusConflict},
		{"no membership", http.MethodPost, noMembership, `{"grant_id":"` + uuid.NewString() + `"}`, http.StatusConflict},
		{"missing id", http.MethodPost, full, `{}`, http.StatusBadRequest},
		{"bad uuid", http.MethodPost, full, `{"grant_id":"not-a-uuid"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/breakglass/approve", strings.NewReader(tc.body))
			if tc.session != nil {
				req = req.WithContext(withSession(req.Context(), tc.session))
			}
			rr := httptest.NewRecorder()
			NewBreakglassApproveHandler(&fakeApprover{}).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status %d, want %d", rr.Code, tc.want)
			}
		})
	}
}
