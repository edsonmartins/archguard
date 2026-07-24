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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/jackc/pgx/v5"
)

// TuplePublisher drains the authz_tuple_outbox (0031) into the authz_tuple
// projection store (0032). It runs asynchronously — OUTSIDE any business
// transaction — which is where a projection write is allowed (RFC-0004 §4). Each
// outbox row is applied to the store and marked published in ONE transaction, so
// a row is never lost and never left applied-but-unmarked in a way that would
// re-apply differently. Application is IDEMPOTENT (spec "Reprocessamento"): a
// write that already exists is a no-op, a delete of an absent tuple is a no-op —
// so at-least-once delivery converges to the same store state.
type TuplePublisher struct{}

// NewTuplePublisher builds the publisher.
func NewTuplePublisher() *TuplePublisher { return &TuplePublisher{} }

type outboxRow struct {
	id        string
	op        string
	user      string
	relation  string
	object    string
	condName  *string
	notBefore *time.Time
	expiresAt *time.Time
}

// Publish applies up to batch unpublished outbox rows, oldest first, and returns
// how many were applied. It is safe to call repeatedly (a scheduler ticks it);
// concurrent publishers do not double-apply because each row's publish is guarded
// by the published_at IS NULL predicate.
func (p *TuplePublisher) Publish(ctx context.Context, db Beginner, batch int) (int, error) {
	var rows []outboxRow
	if err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rs, err := tx.Query(ctx, `
			SELECT id::text, op, tuple_user, tuple_relation, tuple_object,
			       condition_name, not_before, expires_at
			FROM authz_tuple_outbox
			WHERE published_at IS NULL
			ORDER BY occurred_at, id
			LIMIT $1`, batch)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var r outboxRow
			if err := rs.Scan(&r.id, &r.op, &r.user, &r.relation, &r.object,
				&r.condName, &r.notBefore, &r.expiresAt); err != nil {
				return err
			}
			rows = append(rows, r)
		}
		return rs.Err()
	}); err != nil {
		return 0, fmt.Errorf("authz_publisher: leitura do outbox falhou: %w", err)
	}

	applied := 0
	for _, r := range rows {
		if err := WithTx(ctx, db, func(tx pgx.Tx) error {
			if err := applyTupleUpdate(ctx, tx, r); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx,
				`UPDATE authz_tuple_outbox SET published_at = now()
				 WHERE id = $1 AND published_at IS NULL`, r.id)
			if err != nil {
				return fmt.Errorf("authz_publisher: marcação de publicado falhou: %w", err)
			}
			if tag.RowsAffected() != 1 {
				// Another publisher took it concurrently — not an error, just skip.
				return errAlreadyPublished
			}
			return nil
		}); err != nil {
			if errors.Is(err, errAlreadyPublished) {
				continue
			}
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// errAlreadyPublished signals that a row was published concurrently; the publisher
// skips it without counting or failing.
var errAlreadyPublished = fmt.Errorf("authz_publisher: linha já publicada concorrentemente")

// applyTupleUpdate applies one outbox row to the projection store idempotently.
// A write inserts the tuple (no-op if present); a delete removes it (no-op if
// absent). organization_id is derived from the tenant-qualified object.
func applyTupleUpdate(ctx context.Context, tx pgx.Tx, r outboxRow) error {
	switch domain.TupleOp(r.op) {
	case domain.TupleWrite:
		org, ok := domain.RefOrg(r.object)
		if !ok {
			return fmt.Errorf("authz_publisher: objeto sem tenant %q", r.object)
		}
		// A conditioned tuple (a grant) carries its window; ON CONFLICT refreshes
		// the window so a re-granted concession updates its validity in the store.
		_, err := tx.Exec(ctx, `
			INSERT INTO authz_tuple
				(organization_id, tuple_user, tuple_relation, tuple_object,
				 condition_name, not_before, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tuple_user, tuple_relation, tuple_object)
			DO UPDATE SET condition_name = EXCLUDED.condition_name,
			              not_before     = EXCLUDED.not_before,
			              expires_at     = EXCLUDED.expires_at`,
			org, r.user, r.relation, r.object, r.condName, r.notBefore, r.expiresAt)
		if err != nil {
			return fmt.Errorf("authz_publisher: aplicação de write falhou: %w", err)
		}
		return nil
	case domain.TupleDelete:
		_, err := tx.Exec(ctx, `
			DELETE FROM authz_tuple
			WHERE tuple_user = $1 AND tuple_relation = $2 AND tuple_object = $3`,
			r.user, r.relation, r.object)
		if err != nil {
			return fmt.Errorf("authz_publisher: aplicação de delete falhou: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("authz_publisher: operação desconhecida %q", r.op)
	}
}
