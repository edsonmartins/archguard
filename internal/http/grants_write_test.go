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

type fakeGrantRevoker struct {
	err      error
	gotActor RevokeActor
	gotOrg   uuid.UUID
	gotGrant uuid.UUID
}

func (f *fakeGrantRevoker) RevokeGrant(_ context.Context, actor RevokeActor, org, grant uuid.UUID) error {
	f.gotActor, f.gotOrg, f.gotGrant = actor, org, grant
	return f.err
}

func grantRevokeRequest(session *domain.AuthSession, bodyJSON string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/grants/revoke", strings.NewReader(bodyJSON))
	return req.WithContext(withSession(req.Context(), session))
}

func TestGrantRevokeSucceeds(t *testing.T) {
	orgID := uuid.New()
	grant := uuid.New()
	session := sessionWithOrg(orgID)
	revoker := &fakeGrantRevoker{}
	rr := httptest.NewRecorder()
	NewGrantRevokeHandler(revoker).ServeHTTP(rr, grantRevokeRequest(session, `{"grant_id":"`+grant.String()+`"}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// The actor and tenant come from the SESSION (INV-1/INV-5); the grant from the body.
	if revoker.gotActor.IdentityID != session.IdentityID {
		t.Fatalf("actor %v, want session identity %v", revoker.gotActor.IdentityID, session.IdentityID)
	}
	if revoker.gotOrg != orgID || revoker.gotGrant != grant {
		t.Fatalf("revoked %v in %v, want %v in %v", revoker.gotGrant, revoker.gotOrg, grant, orgID)
	}
	if !strings.Contains(rr.Body.String(), `"revoked":true`) {
		t.Fatalf("body should confirm revocation: %s", rr.Body.String())
	}
}

func TestGrantRevokeNotFoundIs404(t *testing.T) {
	session := sessionWithOrg(uuid.New())
	rr := httptest.NewRecorder()
	NewGrantRevokeHandler(&fakeGrantRevoker{err: ErrGrantNotFound}).
		ServeHTTP(rr, grantRevokeRequest(session, `{"grant_id":"`+uuid.NewString()+`"}`))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("not found: status %d, want 404", rr.Code)
	}
}

func TestGrantRevokeNotActiveIs409(t *testing.T) {
	session := sessionWithOrg(uuid.New())
	rr := httptest.NewRecorder()
	NewGrantRevokeHandler(&fakeGrantRevoker{err: ErrGrantNotActive}).
		ServeHTTP(rr, grantRevokeRequest(session, `{"grant_id":"`+uuid.NewString()+`"}`))
	if rr.Code != http.StatusConflict {
		t.Fatalf("not active: status %d, want 409", rr.Code)
	}
}

func TestGrantRevokeValidation(t *testing.T) {
	orgID := uuid.New()
	cases := []struct {
		name    string
		method  string
		session *domain.AuthSession
		body    string
		want    int
	}{
		{"wrong method", http.MethodGet, sessionWithOrg(orgID), `{"grant_id":"` + uuid.NewString() + `"}`, http.StatusMethodNotAllowed},
		{"no session", http.MethodPost, nil, `{"grant_id":"x"}`, http.StatusUnauthorized},
		{"no active tenant", http.MethodPost, &domain.AuthSession{IdentityID: uuid.New(), Status: domain.SessionPendingSelection}, `{"grant_id":"` + uuid.NewString() + `"}`, http.StatusConflict},
		{"missing id", http.MethodPost, sessionWithOrg(orgID), `{}`, http.StatusBadRequest},
		{"bad uuid", http.MethodPost, sessionWithOrg(orgID), `{"grant_id":"not-a-uuid"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/grants/revoke", strings.NewReader(tc.body))
			if tc.session != nil {
				req = req.WithContext(withSession(req.Context(), tc.session))
			}
			rr := httptest.NewRecorder()
			NewGrantRevokeHandler(&fakeGrantRevoker{}).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status %d, want %d", rr.Code, tc.want)
			}
		})
	}
}
