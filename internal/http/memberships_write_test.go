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

type fakeRevoker struct {
	sessions      int
	err           error
	gotActor      RevokeActor
	gotOrg        uuid.UUID
	gotMembership uuid.UUID
}

func (f *fakeRevoker) RevokeMembership(_ context.Context, actor RevokeActor, org, membership uuid.UUID) (int, error) {
	f.gotActor, f.gotOrg, f.gotMembership = actor, org, membership
	return f.sessions, f.err
}

func revokeRequest(session *domain.AuthSession, bodyJSON string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/memberships/revoke", strings.NewReader(bodyJSON))
	return req.WithContext(withSession(req.Context(), session))
}

func TestMembershipRevokeSucceeds(t *testing.T) {
	orgID := uuid.New()
	target := uuid.New()
	session := sessionWithOrg(orgID)
	revoker := &fakeRevoker{sessions: 3}
	rr := httptest.NewRecorder()
	NewMembershipRevokeHandler(revoker).ServeHTTP(rr, revokeRequest(session, `{"membership_id":"`+target.String()+`"}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// The actor and tenant come from the SESSION; the target from the body.
	if revoker.gotActor.IdentityID != session.IdentityID {
		t.Fatalf("actor %v, want session identity %v", revoker.gotActor.IdentityID, session.IdentityID)
	}
	if revoker.gotOrg != orgID || revoker.gotMembership != target {
		t.Fatalf("revoked %v in %v, want %v in %v", revoker.gotMembership, revoker.gotOrg, target, orgID)
	}
	if !strings.Contains(rr.Body.String(), `"sessions_ended":3`) {
		t.Fatalf("body should report ended sessions: %s", rr.Body.String())
	}
}

func TestMembershipRevokeNotFoundIs404(t *testing.T) {
	session := sessionWithOrg(uuid.New())
	rr := httptest.NewRecorder()
	NewMembershipRevokeHandler(&fakeRevoker{err: ErrMembershipNotFound}).
		ServeHTTP(rr, revokeRequest(session, `{"membership_id":"`+uuid.NewString()+`"}`))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("not found: status %d, want 404", rr.Code)
	}
}

func TestMembershipRevokeValidation(t *testing.T) {
	orgID := uuid.New()
	cases := []struct {
		name    string
		session *domain.AuthSession
		body    string
		want    int
	}{
		{"no session", nil, `{"membership_id":"x"}`, http.StatusUnauthorized},
		{"no active tenant", &domain.AuthSession{IdentityID: uuid.New(), Status: domain.SessionPendingSelection}, `{"membership_id":"` + uuid.NewString() + `"}`, http.StatusConflict},
		{"missing id", sessionWithOrg(orgID), `{}`, http.StatusBadRequest},
		{"bad uuid", sessionWithOrg(orgID), `{"membership_id":"not-a-uuid"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/memberships/revoke", strings.NewReader(tc.body))
			if tc.session != nil {
				req = req.WithContext(withSession(req.Context(), tc.session))
			}
			rr := httptest.NewRecorder()
			NewMembershipRevokeHandler(&fakeRevoker{}).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status %d, want %d", rr.Code, tc.want)
			}
		})
	}
}
