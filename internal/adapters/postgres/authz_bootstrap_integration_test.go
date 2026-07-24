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
)

// Bootstrap reconstrói o store a partir da fonte de forma determinística: o estado
// resultante é equivalente ao esperado (spec "Reconstrução completa"), e uma
// reconciliação subsequente contra o mesmo esperado não acusa divergência.
func TestAuthzBootstrapRebuildsDeterministically(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
	})

	mkTuple := func() domain.RelationTuple {
		return domain.RelationTuple{
			User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
			Relation: domain.RelOperator,
			Object:   domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
		}
	}
	expected := []domain.RelationTuple{mkTuple(), mkTuple(), mkTuple()}

	// Store parte de lixo (uma tupla que não está no esperado).
	seedTuple(t, pool, org, mkTuple())

	boot := NewAuthzBootstrap()
	n, err := boot.RebuildTenant(ctx, pool, org, expected)
	if err != nil || n != 3 {
		t.Fatalf("RebuildTenant: n=%d err=%v", n, err)
	}

	// O store é exatamente o esperado.
	for _, tup := range expected {
		if !tupleExists(t, pool, tup) {
			t.Fatalf("tupla esperada ausente após rebuild: %+v", tup)
		}
	}
	var total int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM authz_tuple WHERE organization_id = $1", org.String()).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Fatalf("o lixo deveria ter sido varrido; esperava 3 tuplas, veio %d", total)
	}

	// Determinístico: reconstruir de novo mantém o mesmo estado.
	if n2, err := boot.RebuildTenant(ctx, pool, org, expected); err != nil || n2 != 3 {
		t.Fatalf("rebuild repetido: n=%d err=%v", n2, err)
	}

	// Equivalência com a reconciliação: nenhuma divergência contra o mesmo esperado.
	report, err := NewAuthzReconciler().Reconcile(ctx, pool, org, expected)
	if err != nil {
		t.Fatalf("Reconcile pós-bootstrap: %v", err)
	}
	if report.Diverged() {
		t.Fatalf("após bootstrap a reconciliação não deveria divergir: %+v", report)
	}
}

// Uma tupla esperada fora do tenant aborta o rebuild (INV-5) — nada é plantado.
func TestAuthzBootstrapRejectsCrossTenant(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org, other := uuid.New(), uuid.New()
	stray := domain.RelationTuple{
		User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator,
		Object:   domain.Qualify(other, domain.TypeAsset, uuid.New().String()),
	}
	if _, err := NewAuthzBootstrap().RebuildTenant(ctx, pool, org, []domain.RelationTuple{stray}); err == nil {
		t.Fatalf("tupla fora do tenant deveria abortar o rebuild")
	}
}
