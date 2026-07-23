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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeNotifier records notifications and can report itself unavailable.
type fakeNotifier struct {
	available bool
	sent      []Notification
	failNext  bool
}

func (n *fakeNotifier) Notify(_ context.Context, notif Notification) error {
	if n.failNext {
		return errors.New("canal caiu")
	}
	n.sent = append(n.sent, notif)
	return nil
}

func (n *fakeNotifier) Available(context.Context, string) bool { return n.available }

func requestArgs() (uuid.UUID, uuid.UUID, GrantTarget, BreakglassPolicy, time.Time, time.Time) {
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	return uuid.New(), uuid.New(), GrantTarget{Type: "asset", ID: "db-01", Scope: "admin"},
		BreakglassPolicy{RequiredApprovals: 2}, nb, nb.Add(30 * time.Minute)
}

// O alerta é emitido no momento da solicitação, antes de qualquer aprovação
// (cenário "Alerta na solicitação").
func TestBreakglassRequestAlertsImmediately(t *testing.T) {
	n := &fakeNotifier{available: true}
	r := NewBreakglassRequester(n)
	org, sub, target, policy, nb, exp := requestArgs()

	g, err := r.Request(context.Background(), org, sub, target, policy, "prod fora do ar", "INC-77", nb, exp)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if g.Status != GrantRequested {
		t.Fatalf("a solicitação deveria nascer em requested (sem aprovações), veio %s", g.Status)
	}
	if len(n.sent) != 1 || n.sent[0].Kind != NotifyBreakglassRequested {
		t.Fatalf("um alerta breakglass.requested deveria ter sido emitido na solicitação: %+v", n.sent)
	}
	// O alerta traz o incidente mas NÃO a justificativa (evita PII no canal).
	if !strings.Contains(n.sent[0].Detail, "INC-77") || strings.Contains(n.sent[0].Detail, "fora do ar") {
		t.Fatalf("o alerta deveria citar o incidente sem a justificativa: %q", n.sent[0].Detail)
	}
}

// Fail-closed: sem canal de notificação disponível, a solicitação é negada e
// nenhum grant é criado (cenário "Canal indisponível").
func TestBreakglassRequestDeniedWithoutChannel(t *testing.T) {
	n := &fakeNotifier{available: false}
	r := NewBreakglassRequester(n)
	org, sub, target, policy, nb, exp := requestArgs()

	if _, err := r.Request(context.Background(), org, sub, target, policy, "incidente", "INC-1", nb, exp); !errors.Is(err, ErrNoNotificationChannel) {
		t.Fatalf("sem canal: err = %v, quero ErrNoNotificationChannel", err)
	}
	if len(n.sent) != 0 {
		t.Fatalf("nada deveria ter sido notificado sem canal")
	}
}

// Se o alerta não pôde ser entregue, a solicitação falha (não prossegue
// silenciosa).
func TestBreakglassRequestFailsIfAlertUndeliverable(t *testing.T) {
	n := &fakeNotifier{available: true, failNext: true}
	r := NewBreakglassRequester(n)
	org, sub, target, policy, nb, exp := requestArgs()

	if _, err := r.Request(context.Background(), org, sub, target, policy, "incidente", "INC-1", nb, exp); err == nil {
		t.Fatalf("alerta não entregue deveria falhar a solicitação")
	}
}
