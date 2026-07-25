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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeGrantLister struct {
	grants []domain.PrivilegedGrant
	err    error
	gotOrg uuid.UUID
}

func (f *fakeGrantLister) ListActive(_ context.Context, orgID uuid.UUID) ([]domain.PrivilegedGrant, error) {
	f.gotOrg = orgID
	return f.grants, f.err
}

func TestGrantsHandlerListsActiveTenantGrants(t *testing.T) {
	orgID := uuid.New()
	lister := &fakeGrantLister{grants: []domain.PrivilegedGrant{
		{
			ID:                  uuid.New(),
			OrganizationID:      orgID,
			SubjectMembershipID: uuid.New(),
			Target:              domain.GrantTarget{Type: "asset", ID: "db-prod-01", Scope: "readonly"},
			Origin:              domain.GrantOrigin("breakglass"),
			Status:              domain.GrantStatus("active"),
			NotBefore:           time.Unix(1000, 0),
			ExpiresAt:           time.Unix(2000, 0),
		},
	}}
	req := httptest.NewRequest(http.MethodGet, "/grants", nil)
	req = req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
	rr := httptest.NewRecorder()
	NewGrantsHandler(lister).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	if lister.gotOrg != orgID {
		t.Fatalf("listed org %v, want session org %v", lister.gotOrg, orgID)
	}
	var body struct {
		Grants []grantItem `json:"grants"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Grants) != 1 || body.Grants[0].TargetID != "db-prod-01" {
		t.Fatalf("unexpected grants: %+v", body.Grants)
	}
	if body.Grants[0].ExpiresAt != 2000 {
		t.Fatalf("expires_at = %d, want 2000", body.Grants[0].ExpiresAt)
	}
}

func TestGrantsHandlerFailsClosedWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	NewGrantsHandler(&fakeGrantLister{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/grants", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}
}

func TestGrantsHandlerFailsClosedOnError(t *testing.T) {
	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/grants", nil)
	req = req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
	rr := httptest.NewRecorder()
	NewGrantsHandler(&fakeGrantLister{err: errors.New("boom")}).ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("list error: status %d, want 500", rr.Code)
	}
}
