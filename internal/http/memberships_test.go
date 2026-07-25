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
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeTenantLister struct {
	memberships []domain.Membership
	err         error
	gotOrg      uuid.UUID
}

func (f *fakeTenantLister) ListInTenant(_ context.Context, orgID uuid.UUID) ([]domain.Membership, error) {
	f.gotOrg = orgID
	return f.memberships, f.err
}

func sessionWithOrg(orgID uuid.UUID) *domain.AuthSession {
	return &domain.AuthSession{IdentityID: uuid.New(), OrganizationID: &orgID, Status: domain.SessionActive}
}

func TestMembershipsHandlerListsActiveTenant(t *testing.T) {
	orgID := uuid.New()
	lister := &fakeTenantLister{memberships: []domain.Membership{
		{ID: uuid.New(), IdentityID: uuid.New(), OrganizationID: orgID, Status: domain.MembershipActive},
	}}
	req := httptest.NewRequest(http.MethodGet, "/memberships", nil)
	req = req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
	rr := httptest.NewRecorder()
	NewMembershipsHandler(lister).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// Roster must be listed for the SESSION's active org, never a request value.
	if lister.gotOrg != orgID {
		t.Fatalf("listed org %v, want session org %v", lister.gotOrg, orgID)
	}
	var body struct {
		Memberships []membershipItem `json:"memberships"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Memberships) != 1 {
		t.Fatalf("want 1 membership, got %d", len(body.Memberships))
	}
}

func TestMembershipsHandlerFailsClosedWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	NewMembershipsHandler(&fakeTenantLister{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/memberships", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}
}

func TestMembershipsHandlerNoActiveTenant(t *testing.T) {
	session := &domain.AuthSession{IdentityID: uuid.New(), Status: domain.SessionPendingSelection} // no OrganizationID
	req := httptest.NewRequest(http.MethodGet, "/memberships", nil)
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewMembershipsHandler(&fakeTenantLister{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("no active tenant: status %d, want 409", rr.Code)
	}
}

func TestMembershipsHandlerFailsClosedOnError(t *testing.T) {
	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/memberships", nil)
	req = req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
	rr := httptest.NewRecorder()
	NewMembershipsHandler(&fakeTenantLister{err: errors.New("boom")}).ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("list error: status %d, want 500", rr.Code)
	}
}
