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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeWebAuthnStepUp struct {
	options     any
	beginErr    error
	aal         domain.AAL
	finishErr   error
	gotIdentity uuid.UUID
	gotSession  uuid.UUID
	gotAssert   []byte
}

func (f *fakeWebAuthnStepUp) BeginWebAuthn(_ context.Context, identityID, sessionID uuid.UUID) (any, error) {
	f.gotIdentity, f.gotSession = identityID, sessionID
	return f.options, f.beginErr
}

func (f *fakeWebAuthnStepUp) FinishWebAuthn(_ context.Context, identityID, sessionID uuid.UUID, assertion []byte, _ time.Time) (domain.AAL, error) {
	f.gotIdentity, f.gotSession, f.gotAssert = identityID, sessionID, assertion
	return f.aal, f.finishErr
}

func waRequest(method, path string, session *domain.AuthSession, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if session != nil {
		req = req.WithContext(withSession(req.Context(), session))
	}
	return req
}

func TestWebAuthnBeginSucceeds(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	svc := &fakeWebAuthnStepUp{options: map[string]any{"publicKey": map[string]any{"challenge": "abc"}}}
	rr := httptest.NewRecorder()
	NewStepUpWebAuthnHandler(svc).Begin(rr, waRequest(http.MethodPost, "/stepup/webauthn/begin", session, ""))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	// Identity and session come from the injected session, never the request (INV-1).
	if svc.gotIdentity != session.IdentityID || svc.gotSession != session.ID {
		t.Fatalf("begin used %v/%v, want session %v/%v", svc.gotIdentity, svc.gotSession, session.IdentityID, session.ID)
	}
	if !strings.Contains(rr.Body.String(), "publicKey") {
		t.Fatalf("body should carry assertion options: %s", rr.Body.String())
	}
}

func TestWebAuthnFinishSucceeds(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	svc := &fakeWebAuthnStepUp{aal: domain.AAL3}
	rr := httptest.NewRecorder()
	NewStepUpWebAuthnHandler(svc).Finish(rr, waRequest(http.MethodPost, "/stepup/webauthn/finish", session, `{"id":"x","response":{}}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 — body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"aal":"aal3"`) {
		t.Fatalf("body should report the new AAL: %s", rr.Body.String())
	}
	if len(svc.gotAssert) == 0 {
		t.Fatalf("the raw assertion should be forwarded to the service")
	}
}

func TestWebAuthnStepUpValidation(t *testing.T) {
	session := sessionWithMembership(uuid.New())
	cases := []struct {
		name    string
		leg     string // "begin" | "finish"
		method  string
		session *domain.AuthSession
		body    string
		svc     *fakeWebAuthnStepUp
		want    int
	}{
		{"begin wrong method", "begin", http.MethodGet, session, "", &fakeWebAuthnStepUp{}, http.StatusMethodNotAllowed},
		{"begin no session", "begin", http.MethodPost, nil, "", &fakeWebAuthnStepUp{}, http.StatusUnauthorized},
		{"begin no factor", "begin", http.MethodPost, session, "", &fakeWebAuthnStepUp{beginErr: ErrNoStrongFactor}, http.StatusConflict},
		{"begin infra fail", "begin", http.MethodPost, session, "", &fakeWebAuthnStepUp{beginErr: context.DeadlineExceeded}, http.StatusServiceUnavailable},
		{"finish wrong method", "finish", http.MethodGet, session, `{}`, &fakeWebAuthnStepUp{}, http.StatusMethodNotAllowed},
		{"finish no session", "finish", http.MethodPost, nil, `{}`, &fakeWebAuthnStepUp{}, http.StatusUnauthorized},
		{"finish empty body", "finish", http.MethodPost, session, ``, &fakeWebAuthnStepUp{}, http.StatusBadRequest},
		{"finish denied", "finish", http.MethodPost, session, `{"x":1}`, &fakeWebAuthnStepUp{finishErr: ErrStepUpDenied}, http.StatusUnauthorized},
		{"finish no factor", "finish", http.MethodPost, session, `{"x":1}`, &fakeWebAuthnStepUp{finishErr: ErrNoStrongFactor}, http.StatusConflict},
		{"finish infra fail", "finish", http.MethodPost, session, `{"x":1}`, &fakeWebAuthnStepUp{finishErr: context.DeadlineExceeded}, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "/stepup/webauthn/" + tc.leg
			req := waRequest(tc.method, path, tc.session, tc.body)
			rr := httptest.NewRecorder()
			h := NewStepUpWebAuthnHandler(tc.svc)
			if tc.leg == "begin" {
				h.Begin(rr, req)
			} else {
				h.Finish(rr, req)
			}
			if rr.Code != tc.want {
				t.Fatalf("status %d, want %d — body %s", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}
