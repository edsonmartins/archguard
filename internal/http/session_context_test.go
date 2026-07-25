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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

func TestSessionContextHandlerReturnsContext(t *testing.T) {
	orgID := uuid.New()
	memID := uuid.New()
	idID := uuid.New()
	session := &domain.AuthSession{
		ID:             uuid.New(),
		IdentityID:     idID,
		OrganizationID: &orgID,
		MembershipID:   &memID,
		Status:         domain.SessionActive,
		ProvenAAL:      domain.AAL1,
		AuthMethods:    []domain.FactorType{domain.FactorPassword},
	}

	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewSessionContextHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	var body sessionContextResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.IdentityID != idID.String() {
		t.Fatalf("identity_id = %q, want %q", body.IdentityID, idID)
	}
	if body.OrganizationID != orgID.String() {
		t.Fatalf("organization_id = %q, want %q", body.OrganizationID, orgID)
	}
	if body.Status != string(domain.SessionActive) || body.ProvenAAL != string(domain.AAL1) {
		t.Fatalf("status/aal = %q/%q", body.Status, body.ProvenAAL)
	}
	if len(body.AMR) != 1 || body.AMR[0] != string(domain.FactorPassword) {
		t.Fatalf("amr = %v", body.AMR)
	}
}

func TestSessionContextHandlerFailsClosedWithoutSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/session", nil) // no session in context
	rr := httptest.NewRecorder()
	NewSessionContextHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("without a session, status %d, want 401", rr.Code)
	}
}
