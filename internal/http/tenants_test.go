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

type fakeLister struct {
	memberships []domain.Membership
	err         error
	gotID       uuid.UUID
}

func (f *fakeLister) ListByIdentity(_ context.Context, id uuid.UUID) ([]domain.Membership, error) {
	f.gotID = id
	return f.memberships, f.err
}

func TestTenantsHandlerListsCallerTenants(t *testing.T) {
	idID := uuid.New()
	orgA, orgB := uuid.New(), uuid.New()
	lister := &fakeLister{memberships: []domain.Membership{
		{ID: uuid.New(), OrganizationID: orgA, Status: domain.MembershipActive},
		{ID: uuid.New(), OrganizationID: orgB, Status: domain.MembershipActive},
	}}
	session := &domain.AuthSession{IdentityID: idID, OrganizationID: &orgA, Status: domain.SessionActive}

	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewTenantsHandler(lister).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// The handler must list by the SESSION's identity, never a request value.
	if lister.gotID != idID {
		t.Fatalf("listed by %v, want session identity %v", lister.gotID, idID)
	}
	var body struct {
		Tenants []tenantItem `json:"tenants"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Tenants) != 2 {
		t.Fatalf("want 2 tenants, got %d", len(body.Tenants))
	}
	// The active tenant is the session's org, and only it.
	active := 0
	for _, item := range body.Tenants {
		if item.Active {
			active++
			if item.OrganizationID != orgA.String() {
				t.Fatalf("wrong active tenant: %s", item.OrganizationID)
			}
		}
	}
	if active != 1 {
		t.Fatalf("exactly one tenant must be active, got %d", active)
	}
}

func TestTenantsHandlerFailsClosedWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	NewTenantsHandler(&fakeLister{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/tenants", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}
}

func TestTenantsHandlerFailsClosedOnListError(t *testing.T) {
	session := &domain.AuthSession{IdentityID: uuid.New(), Status: domain.SessionActive}
	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewTenantsHandler(&fakeLister{err: errors.New("global access denied")}).ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("list error: status %d, want 500 (fail-closed)", rr.Code)
	}
}
