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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ciclo completo do workflow de recuperação persistido: o alvo abre a
// solicitação; dois pares DISTINTOS aprovam em transações separadas (cada um
// recarregando o estado); ao atingir o limiar a solicitação fica aprovada e pode
// ser consumida (reset). Cenário "Perda de dispositivo".
func TestRecoveryRequestPeerApprovalFlow(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "recov")

	// Peers: duas identidades adicionais.
	peer1 := seedRecoveryIdentity(t, pool)
	peer2 := seedRecoveryIdentity(t, pool)

	req, err := domain.NewRecoveryRequest(fx.other.ID, fx.orgA, fx.other.ID, "perdi o autenticador", 2)
	if err != nil {
		t.Fatalf("NewRecoveryRequest: %v", err)
	}
	repo := NewTenantRepository(pool, fx.tenantScopeA)
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewRecoveryRequestStore(ttx).Create(ctx, req)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cada par aprova numa transação própria, recarregando o estado.
	approve := func(approver uuid.UUID) {
		if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
			store := NewRecoveryRequestStore(ttx)
			loaded, e := store.Get(ctx, req.ID)
			if e != nil {
				return e
			}
			if e := loaded.Approve(approver); e != nil {
				return e
			}
			return store.SaveDecision(ctx, loaded)
		}); err != nil {
			t.Fatalf("aprovação de %s: %v", approver, err)
		}
	}
	approve(peer1)

	// Após uma aprovação, ainda pendente.
	mid := getRecovery(t, pool, repo, req.ID)
	if mid.Status != domain.RecoveryPending || len(mid.Approvals) != 1 {
		t.Fatalf("após 1 aprovação deveria seguir pendente com 1 aprovação: %s/%d", mid.Status, len(mid.Approvals))
	}

	approve(peer2)

	final := getRecovery(t, pool, repo, req.ID)
	if final.Status != domain.RecoveryApproved || len(final.Approvals) != 2 {
		t.Fatalf("após 2 aprovações distintas deveria estar aprovada com 2 aprovações: %s/%d", final.Status, len(final.Approvals))
	}

	// Consumir (reset realizado).
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		store := NewRecoveryRequestStore(ttx)
		loaded, e := store.Get(ctx, req.ID)
		if e != nil {
			return e
		}
		if e := loaded.MarkConsumed(); e != nil {
			return e
		}
		return store.SaveDecision(ctx, loaded)
	}); err != nil {
		t.Fatalf("consumo: %v", err)
	}
	if getRecovery(t, pool, repo, req.ID).Status != domain.RecoveryConsumed {
		t.Fatalf("status final deveria ser consumed")
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM recovery_approval WHERE recovery_request_id = $1", req.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM recovery_request WHERE id = $1", req.ID.String())
		for _, p := range []uuid.UUID{peer1, peer2} {
			_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", p.String())
		}
	})
}

func seedRecoveryIdentity(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(context.Background(), idn); err != nil {
		t.Fatalf("cria identidade par: %v", err)
	}
	return idn.ID
}

func getRecovery(t *testing.T, _ *pgxpool.Pool, repo *TenantRepository, id uuid.UUID) domain.RecoveryRequest {
	t.Helper()
	var out domain.RecoveryRequest
	if err := repo.WithTenantTx(context.Background(), func(ttx *TenantTx) error {
		var e error
		out, e = NewRecoveryRequestStore(ttx).Get(context.Background(), id)
		return e
	}); err != nil {
		t.Fatalf("getRecovery: %v", err)
	}
	return out
}
