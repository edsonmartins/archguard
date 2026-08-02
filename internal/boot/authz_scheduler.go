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

package boot

import (
	"context"
	"time"

	"github.com/beego/beego/v2/core/logs"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// authzPublishInterval is how often the publisher drains the outbox. Short, so a
	// grant/asset mutation projects to the graph within seconds.
	authzPublishInterval = 5 * time.Second
	// authzPublishBatch is the max rows drained per Publish call.
	authzPublishBatch = 200
	// authzMaxDrainsPerTick caps how many batches one tick drains, so a large backlog
	// never monopolizes the goroutine (the remainder drains on the next tick).
	authzMaxDrainsPerTick = 10
	// authzReconcileInterval is how often the reconciler heals the projection against the
	// source of truth. Longer than publishing — it is a safety net (drift + stale access
	// left by bypass paths, T-030c), not the hot path.
	authzReconcileInterval = 5 * time.Minute
)

// StartAuthzPublisher starts the background authorization-projection publisher (M4,
// T-028): every interval it drains the transactional outbox (authz_tuple_outbox) into
// the projection (authz_tuple), from which the PostgresPDP decides. Without it the
// outbox is never applied and the PDP denies on an empty store (fail-closed, but with
// no data). Errors are logged, never fatal — the mutation already committed durably.
//
// Returns a stop function to call on shutdown (cancels the goroutine).
func StartAuthzPublisher(pool *pgxpool.Pool) func() {
	ctx, cancel := context.WithCancel(context.Background())
	pub := postgres.NewTuplePublisher()
	go func() {
		ticker := time.NewTicker(authzPublishInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				drainAuthzOutbox(ctx, pub, pool)
			}
		}
	}()
	return cancel
}

// drainAuthzOutbox publishes pending outbox rows until the backlog is empty or the
// per-tick cap is hit. A publish failure aborts this tick (retried on the next).
func drainAuthzOutbox(ctx context.Context, pub *postgres.TuplePublisher, pool *pgxpool.Pool) {
	for i := 0; i < authzMaxDrainsPerTick; i++ {
		n, err := pub.Publish(ctx, pool, authzPublishBatch)
		if err != nil {
			logs.Warning("authz publisher: publicação da projeção falhou: %v", err)
			return
		}
		if n < authzPublishBatch {
			return // outbox drenado
		}
	}
}

// StartAuthzReconciler starts the background authorization-projection reconciler (M4 Fase F,
// T-031): every interval it rebuilds each tenant's expected tuple set from the source of
// truth and removes stale (extra) tuples from the projection, healing drift and the stale
// access left by paths that bypass the per-mutation projection (T-030c). Missing tuples are
// only ALERTED, never written (amplifying correction is never automatic). Errors are logged,
// never fatal. Returns a stop function.
func StartAuthzReconciler(pool *pgxpool.Pool) func() {
	ctx, cancel := context.WithCancel(context.Background())
	svc := postgres.NewReconcileService(pool)
	go func() {
		ticker := time.NewTicker(authzReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				removed, missing, err := svc.ReconcileAll(ctx)
				if err != nil {
					logs.Warning("authz reconciler: reconciliação falhou: %v", err)
					continue
				}
				if removed > 0 || missing > 0 {
					logs.Info("authz reconciler: %d tupla(s) obsoleta(s) removida(s), %d ausente(s) alertada(s)", removed, missing)
				}
			}
		}
	}()
	return cancel
}
