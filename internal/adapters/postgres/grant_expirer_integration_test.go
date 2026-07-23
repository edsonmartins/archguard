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
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Cenário "Janela expirada": ao vencer a janela, o job expira a concessão ativa,
// REVOGA em cascata as sessões derivadas e AUDITA a expiração — tudo atômico.
func TestGrantExpirerExpiresAndCascades(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "grantexp")
	cleanupAudit(t, pool, fx.orgA)

	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	exp := nb.Add(30 * time.Minute)

	// Concessão break-glass ativada (1 aprovação distinta) — dentro da org A.
	g, err := domain.NewBreakglassRequest(fx.orgA, fx.memA.ID,
		domain.GrantTarget{Type: "asset", ID: "db-prod", Scope: "admin"}, 1, "incidente", "INC-1", nb, exp)
	if err != nil {
		t.Fatalf("NewBreakglassRequest: %v", err)
	}
	if err := g.PassStepUp(domain.AAL3, true); err != nil {
		t.Fatalf("PassStepUp: %v", err)
	}
	if err := g.Approve(uuid.New()); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if g.Status != domain.GrantActive {
		t.Fatalf("pré-condição: concessão deveria estar ativa")
	}
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewPrivilegedGrantStore(ttx).Create(ctx, g)
	}); err != nil {
		t.Fatalf("Create grant: %v", err)
	}

	// Sessão DERIVADA da concessão (inserida diretamente, referenciando o grant).
	sessionID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session
		   (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, privileged_grant_id)
		 VALUES ($1,$2,$3,$4,'active','aal3',1, $5, $6)`,
		sessionID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), nb, g.ID.String()); err != nil {
		t.Fatalf("insere sessão derivada: %v", err)
	}

	// Executa o job com relógio APÓS a expiração.
	clock := func() time.Time { return exp.Add(time.Minute) }
	n, err := NewGrantExpirer(repo, NewAuditWriter(pool, fixedClock()), clock).ExpireDue(adminCtx())
	if err != nil {
		t.Fatalf("ExpireDue: %v", err)
	}
	if n != 1 {
		t.Fatalf("deveria ter expirado 1 concessão, expirou %d", n)
	}

	// A concessão está expirada.
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM privileged_grant WHERE id = $1", g.ID.String()).Scan(&status); err != nil {
		t.Fatalf("leitura da concessão: %v", err)
	}
	if status != "expired" {
		t.Fatalf("concessão deveria estar expired, veio %s", status)
	}
	// A sessão derivada foi revogada (cascata).
	if err := pool.QueryRow(ctx, "SELECT status FROM auth_session WHERE id = $1", sessionID.String()).Scan(&status); err != nil {
		t.Fatalf("leitura da sessão: %v", err)
	}
	if status != "revoked" {
		t.Fatalf("a sessão derivada deveria estar revogada, veio %s", status)
	}
	// A expiração foi auditada.
	if countAction(t, pool, fx.orgA, domain.ActionPrivilegedGrantExpire) != 1 {
		t.Fatalf("a expiração deveria ter sido auditada")
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE privileged_grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM grant_approval WHERE grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM privileged_grant WHERE id = $1", g.ID.String())
	})
}
