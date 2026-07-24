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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func countOutbox(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id = $1", orgID.String()).
		Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}

func writeTuple(org uuid.UUID) domain.TupleUpdate {
	return domain.TupleUpdate{
		Op: domain.TupleWrite,
		Tuple: domain.RelationTuple{
			User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
			Relation: domain.RelOperator,
			Object:   domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
		},
	}
}

// A intenção de sincronizar é atômica com a mutação: rollback dropa as linhas do
// outbox, commit as persiste (RFC-0004 §4 / spec "Mutação de domínio").
func TestAuthzOutboxAtomicWithTransaction(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()

	// Rollback: nada persiste.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := NewAuthzOutbox(tx).Enqueue(ctx, []domain.TupleUpdate{writeTuple(org), writeTuple(org)}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if n := countOutbox(t, pool, org); n != 0 {
		t.Fatalf("rollback deveria dropar as linhas do outbox, restaram %d", n)
	}

	// Commit: as tuplas ficam disponíveis para o publisher.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	if err := NewAuthzOutbox(tx2).Enqueue(ctx, []domain.TupleUpdate{writeTuple(org), writeTuple(org)}); err != nil {
		t.Fatalf("enqueue2: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n := countOutbox(t, pool, org); n != 2 {
		t.Fatalf("commit deveria persistir 2 linhas, veio %d", n)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", org.String())
	})
}

// Uma tupla cross-tenant é rejeitada e NADA é enfileirado (INV-5): a rejeição
// aborta o lote inteiro, que roda na transação da mutação — que então dá rollback.
func TestAuthzOutboxRejectsCrossTenant(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()

	cross := domain.TupleUpdate{
		Op: domain.TupleWrite,
		Tuple: domain.RelationTuple{
			User:     domain.Qualify(orgA, domain.TypeMembership, uuid.New().String()),
			Relation: domain.RelOperator,
			Object:   domain.Qualify(orgB, domain.TypeAsset, uuid.New().String()),
		},
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	err = NewAuthzOutbox(tx).Enqueue(ctx, []domain.TupleUpdate{cross})
	if !errors.Is(err, domain.ErrCrossTenantRelation) {
		t.Fatalf("tupla cross-tenant deveria ser rejeitada, veio %v", err)
	}
}

// Uma tupla não qualificada por tenant é rejeitada antes de entrar no outbox.
func TestAuthzOutboxRejectsUnqualified(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	bad := domain.TupleUpdate{
		Op:    domain.TupleWrite,
		Tuple: domain.RelationTuple{User: "membership:m1", Relation: domain.RelOperator, Object: "asset:a1"},
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := NewAuthzOutbox(tx).Enqueue(ctx, []domain.TupleUpdate{bad}); !errors.Is(err, domain.ErrUnqualifiedRef) {
		t.Fatalf("tupla não qualificada deveria ser rejeitada, veio %v", err)
	}
}
