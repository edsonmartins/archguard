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
	"encoding/json"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuditQueue is the DURABLE asynchronous audit path (RFC-0003 §7, T-009) for
// NON-privileged events (L1/L2): enqueue is a fast INSERT that does not lock the
// chain head, so it does not add contention to the hot login path; a background
// drainer later seals queued events into the chain (audit_event) in order. Loss
// of a queued event is a high-severity incident, so the row is only removed in
// the SAME transaction that seals it into the chain (at-least-once, deduped by
// the chain's own id). Privileged events (L3) do NOT use this queue — they take
// the synchronous fail-closed AuditSink (T-008).
type AuditQueue struct {
	clock Clock
}

// NewAuditQueue builds the queue with a clock (nil → time.Now) used to stamp the
// event's occurred_at at ENQUEUE time — the moment it happened, not the drain.
func NewAuditQueue(clock Clock) *AuditQueue {
	if clock == nil {
		clock = time.Now
	}
	return &AuditQueue{clock: clock}
}

// Enqueue validates and appends a non-privileged event to the durable queue. It
// refuses a privileged (L3) action — those must go through the synchronous sink,
// not the async queue. It runs on any Querier, so it can share the request's
// transaction. The event_id and occurred_at are captured now and preserved
// through the drain.
func (q *AuditQueue) Enqueue(ctx context.Context, db Querier, in domain.AuditEventInput) error {
	if in.Action.RequirePrivileged() {
		return fmt.Errorf("audit: ação privilegiada %q não pode ir para a fila assíncrona (use o AuditSink)", in.Action)
	}
	event, err := domain.NewAuditEvent(in)
	if err != nil {
		return err
	}
	event.OccurredAt = q.clock().UTC()

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: serialização do evento enfileirado falhou: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("audit: geração do id de fila falhou: %w", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO audit_event_queue (id, organization_id, payload) VALUES ($1, $2, $3)`,
		id.String(), event.OrganizationID.String(), payload); err != nil {
		return fmt.Errorf("audit: enfileiramento falhou: %w", err)
	}
	return nil
}

// Drain seals up to batch queued events into the chain, oldest first per
// organization, and returns how many were sealed. Each event is sealed and its
// queue row deleted in ONE transaction, so an event is never lost (it stays
// queued until durably chained) nor double-chained (the delete commits with the
// seal). Ordering by (organization_id, id) preserves per-tenant occurrence
// order, so the chain stays gapless and in order. Intended for a single drainer
// process; returns 0 when the queue is empty.
func (q *AuditQueue) Drain(ctx context.Context, db Beginner, batch int) (int, error) {
	type queued struct {
		id      string
		payload []byte
	}
	// Read the batch fully within one short read transaction, then process each
	// item in its own seal transaction. Reading eagerly avoids holding a cursor
	// open across the per-item writes.
	var items []queued
	if err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id::text, payload FROM audit_event_queue ORDER BY organization_id, id LIMIT $1`, batch)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it queued
			if err := rows.Scan(&it.id, &it.payload); err != nil {
				return err
			}
			items = append(items, it)
		}
		return rows.Err()
	}); err != nil {
		return 0, fmt.Errorf("audit: leitura da fila falhou: %w", err)
	}

	drained := 0
	for _, it := range items {
		var event domain.AuditEvent
		if err := json.Unmarshal(it.payload, &event); err != nil {
			return drained, fmt.Errorf("audit: desserialização do evento %s falhou: %w", it.id, err)
		}
		if err := WithTx(ctx, db, func(tx pgx.Tx) error {
			if _, err := sealEventInTx(ctx, tx, event); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `DELETE FROM audit_event_queue WHERE id = $1`, it.id)
			if err != nil {
				return fmt.Errorf("audit: remoção da fila falhou: %w", err)
			}
			if tag.RowsAffected() != 1 {
				// Another drainer took it; roll back so we do not double-chain.
				return fmt.Errorf("audit: item %s já consumido concorrentemente", it.id)
			}
			return nil
		}); err != nil {
			return drained, err
		}
		drained++
	}
	return drained, nil
}
