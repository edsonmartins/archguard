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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeDecider struct {
	allowed map[string]bool // relation -> allowed
	err     error
}

func (f fakeDecider) Check(_ context.Context, req domain.CheckRequest) (domain.Decision, error) {
	if f.err != nil {
		return domain.Decision{}, f.err
	}
	return domain.Decision{Allowed: f.allowed[req.Tuple.Relation]}, nil
}

func accessRequest(orgID uuid.UUID, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/access/effective"+query, nil)
	return req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
}

func TestAccessHandlerReportsEffectiveAccess(t *testing.T) {
	orgID := uuid.New()
	decider := fakeDecider{allowed: map[string]bool{
		domain.RelCanOpenSession:           true,
		domain.RelCanOpenPrivilegedSession: false, // structural yes, but no active grant
	}}
	rr := httptest.NewRecorder()
	NewAccessHandler(decider).ServeHTTP(rr, accessRequest(orgID, "?membership="+uuid.NewString()+"&asset=db-01"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	var body accessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.CanOpenSession || body.CanOpenPrivilegedSession {
		t.Fatalf("want can_open_session=true, can_open_privileged_session=false; got %+v", body)
	}
}

func TestAccessHandlerPDPUnavailableIsNotDenial(t *testing.T) {
	orgID := uuid.New()
	rr := httptest.NewRecorder()
	// A PDP that cannot decide must be 503, never a 200 "no access" (fail-closed
	// distinguishes could-not-decide from denied).
	NewAccessHandler(fakeDecider{err: domain.ErrPDPUnavailable}).
		ServeHTTP(rr, accessRequest(orgID, "?membership="+uuid.NewString()+"&asset=db-01"))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("PDP unavailable: status %d, want 503", rr.Code)
	}
}

func TestAccessHandlerRequiresParams(t *testing.T) {
	orgID := uuid.New()
	rr := httptest.NewRecorder()
	NewAccessHandler(fakeDecider{}).ServeHTTP(rr, accessRequest(orgID, "?membership=x")) // no asset
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing asset: status %d, want 400", rr.Code)
	}
}

func TestAccessHandlerFailsClosedWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/access/effective?membership=x&asset=y", nil)
	NewAccessHandler(fakeDecider{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}
}
