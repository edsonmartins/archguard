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

func seedTuple(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, tup domain.RelationTuple) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO authz_tuple (organization_id, tuple_user, tuple_relation, tuple_object)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`,
		org.String(), tup.User, tup.Relation, tup.Object); err != nil {
		t.Fatalf("seedTuple: %v", err)
	}
}

// A reconciliação remove a tupla extra (restringe, automático) e apenas alerta a
// ausente (ampliaria — nunca automática): spec "Reconciliação com política
// assimétrica".
func TestAuthzReconcilerAsymmetric(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()

	legit := domain.RelationTuple{
		User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator, Object: domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
	}
	rogue := domain.RelationTuple{
		User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator, Object: domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
	}
	expectedOnly := domain.RelationTuple{
		User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator, Object: domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
	})

	// Store tem: legit (ok) + rogue (não prevista). Esperado: legit + expectedOnly.
	seedTuple(t, pool, org, legit)
	seedTuple(t, pool, org, rogue)

	report, err := NewAuthzReconciler().Reconcile(ctx, pool, org,
		[]domain.RelationTuple{legit, expectedOnly})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(report.Removed) != 1 || report.Removed[0].User != rogue.User {
		t.Fatalf("a tupla não prevista deveria ter sido removida: %+v", report.Removed)
	}
	if len(report.MissingAlerted) != 1 || report.MissingAlerted[0].User != expectedOnly.User {
		t.Fatalf("a tupla esperada ausente deveria ser alertada: %+v", report.MissingAlerted)
	}

	// A rogue saiu do store; a legit permanece; a expectedOnly NÃO foi adicionada.
	if tupleExists(t, pool, rogue) {
		t.Fatalf("rogue deveria ter sido removida do store")
	}
	if !tupleExists(t, pool, legit) {
		t.Fatalf("legit deveria permanecer")
	}
	if tupleExists(t, pool, expectedOnly) {
		t.Fatalf("expectedOnly NÃO deveria ter sido adicionada (ampliação não é automática)")
	}
}
