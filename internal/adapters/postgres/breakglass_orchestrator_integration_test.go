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

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// bgNotifier is a test Notifier with configurable availability.
type bgNotifier struct {
	available bool
	sent      int
}

func (n *bgNotifier) Notify(context.Context, domain.Notification) error { n.sent++; return nil }
func (n *bgNotifier) Available(context.Context, string) bool            { return n.available }

func bgArgs() (domain.GrantTarget, domain.BreakglassPolicy, time.Time, time.Time) {
	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	return domain.GrantTarget{Type: "asset", ID: "db-prod", Scope: "admin"},
		domain.BreakglassPolicy{RequiredApprovals: 2}, nb, nb.Add(30 * time.Minute)
}

// Caminho feliz: canal disponível + principal presente → concessão persistida,
// solicitação auditada e alerta emitido.
func TestBreakglassOrchestratorHappyPath(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "bgok")
	cleanupAudit(t, pool, fx.orgA)

	n := &bgNotifier{available: true}
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	orch := NewBreakglassOrchestrator(repo, domain.NewBreakglassRequester(n), NewAuditWriter(pool, fixedClock()))

	target, policy, nb, exp := bgArgs()
	g, err := orch.Request(adminCtx(), fx.memA.ID, target, policy, "prod fora do ar", "INC-9", nb, exp)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	var status string
	if err := pool.QueryRow(context.Background(), "SELECT status FROM privileged_grant WHERE id = $1", g.ID.String()).Scan(&status); err != nil {
		t.Fatalf("a concessão deveria ter sido persistida: %v", err)
	}
	if status != "requested" {
		t.Fatalf("status = %s, quero requested", status)
	}
	if n.sent != 1 {
		t.Fatalf("um alerta deveria ter sido emitido, foram %d", n.sent)
	}
	if countAction(t, pool, fx.orgA, domain.ActionBreakglassRequest) != 1 {
		t.Fatalf("a solicitação deveria ter sido auditada")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM privileged_grant WHERE id = $1", g.ID.String())
	})
}

// Fail-closed sem canal: negado e nenhuma concessão persistida (cenário
// "Canal indisponível" / T-019).
func TestBreakglassOrchestratorDeniedWithoutChannel(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "bgnochan")

	n := &bgNotifier{available: false}
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	orch := NewBreakglassOrchestrator(repo, domain.NewBreakglassRequester(n), NewAuditWriter(pool, fixedClock()))

	target, policy, nb, exp := bgArgs()
	if _, err := orch.Request(adminCtx(), fx.memA.ID, target, policy, "incidente", "INC-1", nb, exp); !errors.Is(err, domain.ErrNoNotificationChannel) {
		t.Fatalf("sem canal: err = %v, quero ErrNoNotificationChannel", err)
	}
	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM privileged_grant WHERE organization_id = $1", fx.orgA.String()).Scan(&count); err != nil {
		t.Fatalf("contagem: %v", err)
	}
	if count != 0 {
		t.Fatalf("nenhuma concessão deveria ter sido criada sem canal, há %d", count)
	}
}

// Fail-closed sem auditoria: com canal mas SEM principal no contexto, a
// transação (criação + auditoria) desfaz — nenhuma concessão persiste (I-5.4).
func TestBreakglassOrchestratorFailsClosedWithoutAudit(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "bgnoaudit")

	n := &bgNotifier{available: true}
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	orch := NewBreakglassOrchestrator(repo, domain.NewBreakglassRequester(n), NewAuditWriter(pool, fixedClock()))

	target, policy, nb, exp := bgArgs()
	// Contexto SEM principal.
	if _, err := orch.Request(context.Background(), fx.memA.ID, target, policy, "incidente", "INC-1", nb, exp); !errors.Is(err, domain.ErrNoPrincipal) {
		t.Fatalf("sem principal: err = %v, quero ErrNoPrincipal", err)
	}
	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM privileged_grant WHERE organization_id = $1", fx.orgA.String()).Scan(&count); err != nil {
		t.Fatalf("contagem: %v", err)
	}
	if count != 0 {
		t.Fatalf("solicitação não auditável não deveria persistir, há %d", count)
	}
}
