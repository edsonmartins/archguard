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

// Reconciliação com divergência INJETADA (T-017 / RFC-0004 §8): o store recebe
// tuplas a mais (concedem além do banco) e faltando (o banco esperava), e a
// reconciliação as trata de forma assimétrica; uma segunda passada converge.
func TestAuthzReconcileInjectedDivergence(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
	})

	mk := func() domain.RelationTuple {
		return domain.RelationTuple{
			User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
			Relation: domain.RelOperator,
			Object:   domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
		}
	}
	// Fonte da verdade (esperado): correto1, correto2, faltando1, faltando2.
	correct1, correct2 := mk(), mk()
	missing1, missing2 := mk(), mk()
	expected := []domain.RelationTuple{correct1, correct2, missing1, missing2}

	// Store injetado: correto1, correto2 (ok) + rogue1, rogue2 (não previstos).
	rogue1, rogue2 := mk(), mk()
	for _, tup := range []domain.RelationTuple{correct1, correct2, rogue1, rogue2} {
		seedTuple(t, pool, org, tup)
	}

	rec := NewAuthzReconciler()
	report, err := rec.Reconcile(ctx, pool, org, expected)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Assimetria: 2 rogues removidos (restritivo, automático), 2 faltando alertados.
	if len(report.Removed) != 2 {
		t.Fatalf("esperava 2 removidos (rogues), veio %d", len(report.Removed))
	}
	if len(report.MissingAlerted) != 2 {
		t.Fatalf("esperava 2 alertados (faltando), veio %d", len(report.MissingAlerted))
	}

	// Estado do store: corretos permanecem, rogues sumiram, faltando NÃO foram
	// adicionados (ampliação nunca é automática).
	for _, ok := range []domain.RelationTuple{correct1, correct2} {
		if !tupleExists(t, pool, ok) {
			t.Fatalf("tupla correta deveria permanecer: %+v", ok)
		}
	}
	for _, gone := range []domain.RelationTuple{rogue1, rogue2} {
		if tupleExists(t, pool, gone) {
			t.Fatalf("rogue deveria ter sido removida: %+v", gone)
		}
	}
	for _, absent := range []domain.RelationTuple{missing1, missing2} {
		if tupleExists(t, pool, absent) {
			t.Fatalf("faltando NÃO deveria ser adicionada automaticamente: %+v", absent)
		}
	}

	// Segunda passada: os rogues já sumiram; só restam os faltando (ainda alertados),
	// nenhuma remoção nova (convergência do lado restritivo).
	report2, err := rec.Reconcile(ctx, pool, org, expected)
	if err != nil {
		t.Fatalf("Reconcile 2: %v", err)
	}
	if len(report2.Removed) != 0 {
		t.Fatalf("segunda passada não deveria remover nada, removeu %d", len(report2.Removed))
	}
	if len(report2.MissingAlerted) != 2 {
		t.Fatalf("os faltando seguem alertados até revisão humana, veio %d", len(report2.MissingAlerted))
	}
}
