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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeEnroller struct {
	uri       string
	finishErr error
	gotID     uuid.UUID
	gotCode   string
}

func (f *fakeEnroller) BeginTOTP(_ context.Context, id uuid.UUID) (string, error) {
	f.gotID = id
	return f.uri, nil
}

func (f *fakeEnroller) FinishTOTP(_ context.Context, id uuid.UUID, code string) error {
	f.gotID = id
	f.gotCode = code
	return f.finishErr
}

func factorsSession() (*domain.AuthSession, uuid.UUID) {
	id := uuid.New()
	return &domain.AuthSession{IdentityID: id, Status: domain.SessionActive}, id
}

func TestFactorsBeginReturnsProvisioningURI(t *testing.T) {
	session, id := factorsSession()
	enroller := &fakeEnroller{uri: "otpauth://totp/ArchGuard:x?secret=ABC"}
	req := httptest.NewRequest(http.MethodPost, "/factors/totp/begin", nil)
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewFactorsHandler(enroller).BeginTOTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// Enrollment must be for the SESSION's identity, never a request value.
	if enroller.gotID != id {
		t.Fatalf("enrolled %v, want session identity %v", enroller.gotID, id)
	}
	if !strings.Contains(rr.Body.String(), "otpauth://") {
		t.Fatalf("body should carry the provisioning uri: %s", rr.Body.String())
	}
}

func TestFactorsBeginFailsClosedWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	NewFactorsHandler(&fakeEnroller{}).BeginTOTP(rr, httptest.NewRequest(http.MethodPost, "/factors/totp/begin", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}
}

func TestFactorsVerifyEnrolls(t *testing.T) {
	session, _ := factorsSession()
	enroller := &fakeEnroller{}
	req := httptest.NewRequest(http.MethodPost, "/factors/totp/verify", strings.NewReader(`{"code":"123456"}`))
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewFactorsHandler(enroller).FinishTOTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	if enroller.gotCode != "123456" {
		t.Fatalf("code = %q, want 123456", enroller.gotCode)
	}
}

func TestFactorsVerifyRequiresCode(t *testing.T) {
	session, _ := factorsSession()
	req := httptest.NewRequest(http.MethodPost, "/factors/totp/verify", strings.NewReader(`{}`))
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewFactorsHandler(&fakeEnroller{}).FinishTOTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing code: status %d, want 400", rr.Code)
	}
}

func TestFactorsVerifyNoPendingIsConflict(t *testing.T) {
	session, _ := factorsSession()
	req := httptest.NewRequest(http.MethodPost, "/factors/totp/verify", strings.NewReader(`{"code":"123456"}`))
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewFactorsHandler(&fakeEnroller{finishErr: ErrNoPendingEnrollment}).FinishTOTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("no pending: status %d, want 409", rr.Code)
	}
}

func TestFactorsVerifyWrongCodeIsBadRequest(t *testing.T) {
	session, _ := factorsSession()
	req := httptest.NewRequest(http.MethodPost, "/factors/totp/verify", strings.NewReader(`{"code":"000000"}`))
	req = req.WithContext(withSession(req.Context(), session))
	rr := httptest.NewRecorder()
	NewFactorsHandler(&fakeEnroller{finishErr: errors.New("código inválido")}).FinishTOTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("wrong code: status %d, want 400", rr.Code)
	}
}
