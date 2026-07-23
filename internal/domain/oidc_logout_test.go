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

package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLogoutTokenClaims(t *testing.T) {
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	c, err := NewLogoutTokenClaims("iss", "warpgate", "sid-1", "jti-1", at)
	if err != nil {
		t.Fatalf("NewLogoutTokenClaims: %v", err)
	}
	if err := c.WellFormed(); err != nil {
		t.Fatalf("logout token deveria ser WellFormed: %v", err)
	}
	if _, ok := c.Events[BackchannelLogoutEvent]; !ok {
		t.Fatalf("o evento de back-channel logout deveria estar presente")
	}
	// Sem o evento não é um logout token.
	c.Events = nil
	if err := c.WellFormed(); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("logout token sem evento deveria ser recusado: %v", err)
	}
}

type fakeRevoker struct {
	called  bool
	failErr error
}

func (r *fakeRevoker) RevokeSession(context.Context, uuid.UUID) error {
	r.called = true
	return r.failErr
}

type fakeLogoutSigner struct{}

func (fakeLogoutSigner) SignLogoutToken(c LogoutTokenClaims) (string, error) {
	return "signed-" + c.Audience, nil
}

type fakeLogoutNotifier struct {
	sent    map[string]string
	failFor string
}

func (n *fakeLogoutNotifier) SendLogout(_ context.Context, endpoint, token string) error {
	if endpoint == n.failFor {
		return errors.New("componente inalcançável")
	}
	if n.sent == nil {
		n.sent = map[string]string{}
	}
	n.sent[endpoint] = token
	return nil
}

// Logout no ArchGuard: revoga localmente e envia logout aos componentes (cenário
// "Logout no ArchGuard").
func TestLogoutPropagation(t *testing.T) {
	rev := &fakeRevoker{}
	notif := &fakeLogoutNotifier{}
	p := NewLogoutPropagator("iss", fakeLogoutSigner{}, notif, rev)

	clients := []LogoutClient{
		{Audience: "warpgate", Endpoint: "https://wg/logout"},
		{Audience: "guacamole", Endpoint: "https://guac/logout"},
	}
	if err := p.Logout(context.Background(), uuid.New(), "sid-1", clients, time.Now()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !rev.called {
		t.Fatalf("a revogação local deveria ter sido chamada")
	}
	if len(notif.sent) != 2 || notif.sent["https://wg/logout"] != "signed-warpgate" {
		t.Fatalf("logout deveria ter sido enviado a ambos: %+v", notif.sent)
	}
}

// Fail-closed: se a revogação local falha, NADA é enviado.
func TestLogoutFailClosedOnLocalRevoke(t *testing.T) {
	rev := &fakeRevoker{failErr: errors.New("db caiu")}
	notif := &fakeLogoutNotifier{}
	p := NewLogoutPropagator("iss", fakeLogoutSigner{}, notif, rev)

	if err := p.Logout(context.Background(), uuid.New(), "sid-1",
		[]LogoutClient{{Audience: "warpgate", Endpoint: "https://wg/logout"}}, time.Now()); err == nil {
		t.Fatalf("revogação local falha deveria falhar o logout")
	}
	if len(notif.sent) != 0 {
		t.Fatalf("nada deveria ter sido enviado com revogação local falha")
	}
}

// Um componente inalcançável é reportado (para o chamador se apoiar na
// introspecção), mas a revogação local já aconteceu.
func TestLogoutReportsFailedSends(t *testing.T) {
	rev := &fakeRevoker{}
	notif := &fakeLogoutNotifier{failFor: "https://guac/logout"}
	p := NewLogoutPropagator("iss", fakeLogoutSigner{}, notif, rev)

	err := p.Logout(context.Background(), uuid.New(), "sid-1", []LogoutClient{
		{Audience: "warpgate", Endpoint: "https://wg/logout"},
		{Audience: "guacamole", Endpoint: "https://guac/logout"},
	}, time.Now())
	if err == nil {
		t.Fatalf("um envio falho deveria ser reportado")
	}
	// O envio bem-sucedido aconteceu; só o falho é reportado.
	if notif.sent["https://wg/logout"] == "" {
		t.Fatalf("o componente alcançável deveria ter recebido o logout")
	}
	if !rev.called {
		t.Fatalf("a revogação local deveria ter acontecido apesar do envio falho")
	}
}

// EndSessionService: envio falho a componente é NÃO-fatal (logout do usuário
// vale); falha na revogação local É fatal.
func TestEndSessionService(t *testing.T) {
	reg, _ := DefaultClientRegistry()

	// Envio falho a um componente com logout: não-fatal.
	notif := &fakeLogoutNotifier{failFor: "https://warpgate.archgate.internal/@warpgate/oidc/logout"}
	prop := NewLogoutPropagator("iss", fakeLogoutSigner{}, notif, &fakeRevoker{})
	svc := NewEndSessionService(prop, reg, nil)
	if err := svc.EndSession(context.Background(), uuid.New(), "sid-1"); err != nil {
		t.Fatalf("envio falho não deveria falhar o logout do usuário: %v", err)
	}

	// Revogação local falha: fatal.
	prop2 := NewLogoutPropagator("iss", fakeLogoutSigner{}, &fakeLogoutNotifier{}, &fakeRevoker{failErr: errors.New("db")})
	svc2 := NewEndSessionService(prop2, reg, nil)
	if err := svc2.EndSession(context.Background(), uuid.New(), "sid-1"); !errors.Is(err, ErrLocalRevocationFailed) {
		t.Fatalf("falha de revogação local deveria ser fatal: %v", err)
	}
}
