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
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeSwitcher struct {
	next   *domain.AuthSession
	err    error
	gotOrg uuid.UUID
	called bool
}

func (f *fakeSwitcher) Switch(_ context.Context, _ *domain.AuthSession, org uuid.UUID) (*domain.AuthSession, error) {
	f.called = true
	f.gotOrg = org
	return f.next, f.err
}

func switchSession() *domain.AuthSession {
	id := uuid.New()
	org := uuid.New()
	mem := uuid.New()
	return &domain.AuthSession{
		ID: uuid.New(), IdentityID: id,
		OrganizationID: &org, MembershipID: &mem,
		Status: domain.SessionActive, ProvenAAL: domain.AAL1,
	}
}

func doSwitch(t *testing.T, sw TenantSwitcher, withSess bool, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/session/tenant", strings.NewReader(body))
	if withSess {
		req = req.WithContext(withSession(req.Context(), switchSession()))
	}
	rr := httptest.NewRecorder()
	NewSessionSwitchHandler(sw).ServeHTTP(rr, req)
	return rr
}

func TestSessionSwitchSuccess(t *testing.T) {
	newOrg := uuid.New()
	next := switchSession()
	next.OrganizationID = &newOrg
	sw := &fakeSwitcher{next: next}

	target := uuid.New()
	rr := doSwitch(t, sw, true, http.MethodPost, `{"organization_id":"`+target.String()+`"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if sw.gotOrg != target {
		t.Fatalf("org repassada = %s, want %s", sw.gotOrg, target)
	}
	var resp sessionContextResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decodificar resposta: %v", err)
	}
	if resp.OrganizationID != newOrg.String() {
		t.Fatalf("contexto retornado org = %q, want o novo tenant %q", resp.OrganizationID, newOrg)
	}
}

// TestSessionSwitchStepUp: destino mais restritivo ⇒ 401 RFC 9470 (desafio de step-up).
func TestSessionSwitchStepUp(t *testing.T) {
	sw := &fakeSwitcher{err: domain.ErrStepUpRequired}
	rr := doSwitch(t, sw, true, http.MethodPost, `{"organization_id":"`+uuid.New().String()+`"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (step-up)", rr.Code)
	}
	if wa := rr.Header().Get("WWW-Authenticate"); !strings.Contains(wa, "insufficient_user_authentication") {
		t.Fatalf("WWW-Authenticate = %q, want desafio de step-up", wa)
	}
}

func TestSessionSwitchDenialsAndFailClosed(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"mesmo tenant", domain.ErrSameTenant, http.StatusConflict},
		{"não é membro", ErrDestNotMember, http.StatusForbidden},
		{"membership estrangeiro", domain.ErrForeignMembership, http.StatusForbidden},
		{"sessão revogada", domain.ErrSessionRevoked, http.StatusUnauthorized},
		{"política indisponível (fail-closed)", domain.ErrDestinationPolicyUnavailable, http.StatusServiceUnavailable},
		{"auditoria indisponível (fail-closed)", domain.ErrSwitchAuditUnavailable, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw := &fakeSwitcher{err: c.err}
			rr := doSwitch(t, sw, true, http.MethodPost, `{"organization_id":"`+uuid.New().String()+`"}`)
			if rr.Code != c.want {
				t.Fatalf("status = %d, want %d", rr.Code, c.want)
			}
		})
	}
}

func TestSessionSwitchGuards(t *testing.T) {
	// Sem sessão no contexto ⇒ fail-closed 401, nunca troca.
	sw := &fakeSwitcher{next: switchSession()}
	if rr := doSwitch(t, sw, false, http.MethodPost, `{"organization_id":"`+uuid.New().String()+`"}`); rr.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão: status = %d, want 401", rr.Code)
	}
	if sw.called {
		t.Fatal("switcher NÃO deveria ser chamado sem sessão")
	}
	// Método errado ⇒ 405.
	if rr := doSwitch(t, &fakeSwitcher{}, true, http.MethodGet, ""); rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: status = %d, want 405", rr.Code)
	}
	// Corpo inválido ⇒ 400.
	if rr := doSwitch(t, &fakeSwitcher{}, true, http.MethodPost, `{`); rr.Code != http.StatusBadRequest {
		t.Fatalf("corpo inválido: status = %d, want 400", rr.Code)
	}
	// organization_id não-UUID ⇒ 400.
	if rr := doSwitch(t, &fakeSwitcher{}, true, http.MethodPost, `{"organization_id":"xyz"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("org inválida: status = %d, want 400", rr.Code)
	}
}
