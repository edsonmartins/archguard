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

	"github.com/jackc/pgx/v5"
)

// Beginner starts a transaction. *pgxpool.Pool satisfies it, and so can a fake
// in tests — WithTx depends on this narrow interface, not on the pool directly.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTx runs fn inside a single transaction (RFC-0002 §5: one transaction per
// business operation). The transaction is committed if fn returns nil, and
// rolled back on any error or panic. The deferred Rollback is a no-op after a
// successful Commit, so it is always safe.
//
// Do NOT make remote calls inside fn (RFC-0004 §4: use the transactional
// outbox) — a transaction must not depend on the network.
func WithTx(ctx context.Context, db Beginner, fn func(pgx.Tx) error) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return err
}
