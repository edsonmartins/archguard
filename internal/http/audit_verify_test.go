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

type fakeVerifier struct {
	rep domain.VerifyReport
	err error
}

func (f fakeVerifier) VerifyOrganization(_ context.Context, _ uuid.UUID) (domain.VerifyReport, error) {
	return f.rep, f.err
}

func get(t *testing.T, h http.Handler, session *domain.AuthSession) (*httptest.ResponseRecorder, verifyResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/audit/verify", nil)
	if session != nil {
		req = req.WithContext(withSession(req.Context(), session))
	}
	h.ServeHTTP(rec, req)
	var body verifyResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestAuditVerifyHandlerIntact(t *testing.T) {
	h := NewAuditVerifyHandler(fakeVerifier{rep: domain.VerifyReport{OK: true, EventsChecked: 3, SealsChecked: 1}})
	rec, body := get(t, h, sessionWithOrg(uuid.New()))
	if rec.Code != http.StatusOK || !body.OK || body.EventsChecked != 3 {
		t.Fatalf("íntegra: code=%d body=%+v", rec.Code, body)
	}
}

func TestAuditVerifyHandlerDivergence(t *testing.T) {
	h := NewAuditVerifyHandler(fakeVerifier{rep: domain.VerifyReport{
		OK: false, FirstDivergence: 2, Kind: domain.DivergenceAltered, Detail: "seq 2",
	}})
	rec, body := get(t, h, sessionWithOrg(uuid.New()))
	// Divergência ⇒ 409 (um monitor alerta só pelo status).
	if rec.Code != http.StatusConflict || body.OK || body.Kind != "altered" || body.FirstDivergence != 2 {
		t.Fatalf("divergência: code=%d body=%+v", rec.Code, body)
	}
}

// O org vem da SESSÃO (INV-5): sem tenant ativo é 409, não uma verificação de org arbitrário.
func TestAuditVerifyHandlerNoActiveTenant(t *testing.T) {
	h := NewAuditVerifyHandler(fakeVerifier{rep: domain.VerifyReport{OK: true}})
	rec, _ := get(t, h, &domain.AuthSession{IdentityID: uuid.New(), Status: domain.SessionPendingSelection})
	if rec.Code != http.StatusConflict {
		t.Fatalf("sem tenant ativo: code=%d, quero 409", rec.Code)
	}
	recNoSession, _ := get(t, h, nil)
	if recNoSession.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão: code=%d, quero 401", recNoSession.Code)
	}
}

// Fail-closed: verificação que não pôde rodar vira 500, nunca "íntegra".
func TestAuditVerifyHandlerVerifierError(t *testing.T) {
	h := NewAuditVerifyHandler(fakeVerifier{err: errors.New("banco fora")})
	rec, _ := get(t, h, sessionWithOrg(uuid.New()))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("erro de verificação: code=%d, quero 500", rec.Code)
	}
}

func TestAuditVerifyHandlerMethodNotAllowed(t *testing.T) {
	h := NewAuditVerifyHandler(fakeVerifier{rep: domain.VerifyReport{OK: true}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/audit/verify", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: code=%d, quero 405", rec.Code)
	}
}

// INV-8: a operação declara seu nível de garantia (L3).
func TestAuditVerifyDeclaresL3(t *testing.T) {
	if AuditVerifyAssuranceLevel != domain.L3 {
		t.Fatalf("verificação da trilha deveria ser L3, é %s", AuditVerifyAssuranceLevel)
	}
}
