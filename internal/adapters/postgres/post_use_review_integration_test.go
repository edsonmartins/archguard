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

// Cenário "Revisão pendente": um break-glass encerrado sem revisão aparece como
// pendência; ao registrar a revisão, deixa de ser pendência.
func TestPostUseReviewPendingUntilRecorded(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "review")

	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	g, err := domain.NewBreakglassRequest(fx.orgA, fx.memA.ID,
		domain.GrantTarget{Type: "asset", ID: "db", Scope: "admin"}, 1, "inc", "INC-1", nb, nb.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("NewBreakglassRequest: %v", err)
	}
	_ = g.PassStepUp(domain.AAL3, true)
	_ = g.Approve(uuid.New())
	_ = g.Expire(nb.Add(31 * time.Minute)) // encerrado (expired) → requer revisão

	repo := NewTenantRepository(pool, fx.tenantScopeA)
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewPrivilegedGrantStore(ttx).Create(ctx, g)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Aparece como pendência.
	var pending []uuid.UUID
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		pending, e = NewPrivilegedGrantStore(ttx).ListPendingReviews(ctx)
		return e
	}); err != nil {
		t.Fatalf("ListPendingReviews: %v", err)
	}
	if len(pending) != 1 || pending[0] != g.ID {
		t.Fatalf("a concessão encerrada deveria estar pendente de revisão, veio %v", pending)
	}

	// Registra a revisão.
	review, err := domain.NewPostUseReview(g, uuid.New(), "acesso legítimo, incidente confirmado")
	if err != nil {
		t.Fatalf("NewPostUseReview: %v", err)
	}
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewPrivilegedGrantStore(ttx).RecordReview(ctx, review)
	}); err != nil {
		t.Fatalf("RecordReview: %v", err)
	}

	// Não é mais pendência.
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		pending, e = NewPrivilegedGrantStore(ttx).ListPendingReviews(ctx)
		return e
	}); err != nil {
		t.Fatalf("ListPendingReviews pós-revisão: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("após a revisão não deveria haver pendência, veio %v", pending)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM breakglass_review WHERE grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM grant_approval WHERE grant_id = $1", g.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM privileged_grant WHERE id = $1", g.ID.String())
	})
}
