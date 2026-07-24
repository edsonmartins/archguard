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

func tupleExists(t *testing.T, pool *pgxpool.Pool, tup domain.RelationTuple) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM authz_tuple
		WHERE tuple_user = $1 AND tuple_relation = $2 AND tuple_object = $3`,
		tup.User, tup.Relation, tup.Object).Scan(&n); err != nil {
		t.Fatalf("tupleExists: %v", err)
	}
	return n > 0
}

func enqueueUpdate(t *testing.T, pool *pgxpool.Pool, u domain.TupleUpdate) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := NewAuthzOutbox(tx).Enqueue(ctx, []domain.TupleUpdate{u}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// O publisher aplica a projeção e é idempotente: reenfileirar o mesmo write e
// republicar leva ao MESMO estado (spec "Reprocessamento"); um delete remove.
func TestTuplePublisherAppliesIdempotently(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	tup := domain.RelationTuple{
		User:     domain.Qualify(org, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator,
		Object:   domain.Qualify(org, domain.TypeAsset, uuid.New().String()),
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", org.String())
	})

	pub := NewTuplePublisher()

	// 1) write -> publica -> tupla presente.
	enqueueUpdate(t, pool, domain.TupleUpdate{Op: domain.TupleWrite, Tuple: tup})
	if n, err := pub.Publish(ctx, pool, 100); err != nil || n != 1 {
		t.Fatalf("publish write: n=%d err=%v", n, err)
	}
	if !tupleExists(t, pool, tup) {
		t.Fatalf("tupla deveria existir após publicação do write")
	}

	// 2) republicar não faz nada (linha já publicada).
	if n, err := pub.Publish(ctx, pool, 100); err != nil || n != 0 {
		t.Fatalf("republicação deveria ser no-op: n=%d err=%v", n, err)
	}

	// 3) reenfileirar o MESMO write e publicar de novo -> idempotente (ainda 1 linha).
	enqueueUpdate(t, pool, domain.TupleUpdate{Op: domain.TupleWrite, Tuple: tup})
	if n, err := pub.Publish(ctx, pool, 100); err != nil || n != 1 {
		t.Fatalf("publish write repetido: n=%d err=%v", n, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM authz_tuple
		WHERE tuple_user = $1 AND tuple_relation = $2 AND tuple_object = $3`,
		tup.User, tup.Relation, tup.Object).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("write idempotente deveria manter 1 linha, veio %d", count)
	}

	// 4) delete -> publica -> tupla removida.
	enqueueUpdate(t, pool, domain.TupleUpdate{Op: domain.TupleDelete, Tuple: tup})
	if n, err := pub.Publish(ctx, pool, 100); err != nil || n != 1 {
		t.Fatalf("publish delete: n=%d err=%v", n, err)
	}
	if tupleExists(t, pool, tup) {
		t.Fatalf("tupla deveria ter sido removida pelo delete")
	}

	// 5) delete de tupla ausente é no-op idempotente.
	enqueueUpdate(t, pool, domain.TupleUpdate{Op: domain.TupleDelete, Tuple: tup})
	if n, err := pub.Publish(ctx, pool, 100); err != nil || n != 1 {
		t.Fatalf("publish delete ausente: n=%d err=%v", n, err)
	}
	if tupleExists(t, pool, tup) {
		t.Fatalf("delete de ausente não deveria recriar a tupla")
	}
}
