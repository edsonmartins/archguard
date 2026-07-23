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

// O ciclo do break-glass é auditado: aprovação, revogação (com cascata) e
// revisão pós-uso — cada um atômico com seu evento (T-017).
func TestPrivilegedAccessServiceAuditsCycle(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := adminCtx()
	fx := makeSessionFixture(t, pool, "pacycle")
	cleanupAudit(t, pool, fx.orgA)

	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	exp := nb.Add(time.Hour)
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	svc := NewPrivilegedAccessService(repo, NewAuditWriter(pool, fixedClock()))

	// Concessão em awaiting_approval (após step-up), 1 aprovação ativa.
	g, err := domain.NewBreakglassRequest(fx.orgA, fx.memA.ID,
		domain.GrantTarget{Type: "asset", ID: "db", Scope: "admin"}, 1, "inc", "INC-1", nb, exp)
	if err != nil {
		t.Fatalf("NewBreakglassRequest: %v", err)
	}
	_ = g.PassStepUp(domain.AAL3, true)
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewPrivilegedGrantStore(ttx).Create(ctx, g)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Aprovação → ativa + auditada.
	approved, err := svc.Approve(ctx, g.ID, uuid.New())
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.Status != domain.GrantActive {
		t.Fatalf("após aprovação deveria estar ativa, veio %s", approved.Status)
	}
	if countAction(t, pool, fx.orgA, domain.ActionBreakglassApprove) != 1 {
		t.Fatalf("a aprovação deveria ter sido auditada")
	}

	// Sessão derivada, depois revogação → cascata + auditada.
	sessionID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, privileged_grant_id)
		 VALUES ($1,$2,$3,$4,'active','aal3',1,$5,$6)`,
		sessionID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), nb, g.ID.String()); err != nil {
		t.Fatalf("insere sessão derivada: %v", err)
	}
	if err := svc.Revoke(ctx, g.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	var sessStatus string
	_ = pool.QueryRow(ctx, "SELECT status FROM auth_session WHERE id = $1", sessionID.String()).Scan(&sessStatus)
	if sessStatus != "revoked" {
		t.Fatalf("a sessão derivada deveria ter sido revogada em cascata, veio %s", sessStatus)
	}
	if countAction(t, pool, fx.orgA, domain.ActionPrivilegedGrantRevoke) != 1 {
		t.Fatalf("a revogação deveria ter sido auditada")
	}

	// Revisão pós-uso → auditada.
	revoked, err := getGrant(t, pool, repo, g.ID)
	if err != nil {
		t.Fatalf("getGrant: %v", err)
	}
	review, err := domain.NewPostUseReview(revoked, uuid.New(), "acesso legítimo")
	if err != nil {
		t.Fatalf("NewPostUseReview: %v", err)
	}
	if err := svc.RecordReview(ctx, review); err != nil {
		t.Fatalf("RecordReview: %v", err)
	}
	if countAction(t, pool, fx.orgA, domain.ActionPrivilegedReview) != 1 {
		t.Fatalf("a revisão deveria ter sido auditada")
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM breakglass_review WHERE grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE privileged_grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM grant_approval WHERE grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM privileged_grant WHERE id = $1", g.ID.String())
	})
}

func getGrant(t *testing.T, _ interface{}, repo *TenantRepository, id uuid.UUID) (domain.PrivilegedGrant, error) {
	t.Helper()
	var out domain.PrivilegedGrant
	err := repo.WithTenantTx(context.Background(), func(ttx *TenantTx) error {
		var e error
		out, e = NewPrivilegedGrantStore(ttx).Get(context.Background(), id)
		return e
	})
	return out, err
}
