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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeStepUp struct {
	aal     domain.AAL
	err     error
	gotID   uuid.UUID
	gotSID  uuid.UUID
	gotCode string
}

func (f *fakeStepUp) StepUpTOTP(_ context.Context, id, sid uuid.UUID, code string, _ time.Time) (domain.AAL, error) {
	f.gotID, f.gotSID, f.gotCode = id, sid, code
	return f.aal, f.err
}

func stepUpRequest(session *domain.AuthSession, bodyJSON string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/stepup/totp", strings.NewReader(bodyJSON))
	return req.WithContext(withSession(req.Context(), session))
}

func TestStepUpRaisesAssurance(t *testing.T) {
	session := &domain.AuthSession{ID: uuid.New(), IdentityID: uuid.New(), Status: domain.SessionActive}
	svc := &fakeStepUp{aal: domain.AAL2}
	rr := httptest.NewRecorder()
	NewStepUpHandler(svc).TOTP(rr, stepUpRequest(session, `{"code":"123456"}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// The step-up must target the SESSION's identity and session, never a request value.
	if svc.gotID != session.IdentityID || svc.gotSID != session.ID {
		t.Fatalf("stepped up %v/%v, want %v/%v", svc.gotID, svc.gotSID, session.IdentityID, session.ID)
	}
	if !strings.Contains(rr.Body.String(), string(domain.AAL2)) {
		t.Fatalf("body should report the new aal: %s", rr.Body.String())
	}
}

func TestStepUpWrongCodeIsUnauthorized(t *testing.T) {
	session := &domain.AuthSession{ID: uuid.New(), IdentityID: uuid.New(), Status: domain.SessionActive}
	rr := httptest.NewRecorder()
	NewStepUpHandler(&fakeStepUp{err: ErrStepUpDenied}).TOTP(rr, stepUpRequest(session, `{"code":"000000"}`))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: status %d, want 401", rr.Code)
	}
}

func TestStepUpNoFactorIsConflict(t *testing.T) {
	session := &domain.AuthSession{ID: uuid.New(), IdentityID: uuid.New(), Status: domain.SessionActive}
	rr := httptest.NewRecorder()
	NewStepUpHandler(&fakeStepUp{err: ErrNoStrongFactor}).TOTP(rr, stepUpRequest(session, `{"code":"123456"}`))
	if rr.Code != http.StatusConflict {
		t.Fatalf("no factor: status %d, want 409", rr.Code)
	}
}

func TestStepUpInfraErrorFailsClosed(t *testing.T) {
	session := &domain.AuthSession{ID: uuid.New(), IdentityID: uuid.New(), Status: domain.SessionActive}
	rr := httptest.NewRecorder()
	NewStepUpHandler(&fakeStepUp{err: errors.New("vault down")}).TOTP(rr, stepUpRequest(session, `{"code":"123456"}`))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("infra error: status %d, want 503 (fail-closed)", rr.Code)
	}
}

func TestStepUpRequiresSessionAndCode(t *testing.T) {
	// No session in context.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/stepup/totp", strings.NewReader(`{"code":"1"}`))
	NewStepUpHandler(&fakeStepUp{}).TOTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}

	// Session but no code.
	session := &domain.AuthSession{ID: uuid.New(), IdentityID: uuid.New(), Status: domain.SessionActive}
	rr = httptest.NewRecorder()
	NewStepUpHandler(&fakeStepUp{}).TOTP(rr, stepUpRequest(session, `{}`))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("no code: status %d, want 400", rr.Code)
	}
}
