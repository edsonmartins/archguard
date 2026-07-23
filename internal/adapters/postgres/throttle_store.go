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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ThrottleStore is the identity-scoped store for progressive-lockout state
// (T-014). Built on an IdentityTx, it carries the explicit identity_id predicate
// (Barreira 1) and the SET LOCAL identity setting the RLS policy reads (Barreira
// 2). It refuses to touch another identity's throttle.
type ThrottleStore struct {
	itx *IdentityTx
}

// NewThrottleStore builds the store on an open identity transaction.
func NewThrottleStore(itx *IdentityTx) *ThrottleStore {
	return &ThrottleStore{itx: itx}
}

// Get returns the identity's current throttle state, or the zero Throttle (a
// clean subject) when no row exists. It refuses an identity other than the
// store's scope. A query FAILURE is an error the caller denies on (INV-6) — the
// throttle can never be bypassed by an unreadable state.
func (s *ThrottleStore) Get(ctx context.Context, identityID uuid.UUID) (domain.Throttle, error) {
	if identityID != s.itx.scope.IdentityID() {
		return domain.Throttle{}, fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossIdentityWrite, identityID, s.itx.scope.IdentityID())
	}
	const q = `SELECT failures, locked_until FROM auth_throttle WHERE identity_id = $1`
	var t domain.Throttle
	var lockedUntil *time.Time
	err := s.itx.tx.QueryRow(ctx, q, identityID.String()).Scan(&t.Failures, &lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Throttle{}, nil
	}
	if err != nil {
		return domain.Throttle{}, fmt.Errorf("postgres: leitura de throttle falhou: %w", err)
	}
	if lockedUntil != nil {
		t.LockedUntil = *lockedUntil
	}
	return t, nil
}

// Save upserts the identity's throttle state. It refuses an identity other than
// the store's scope.
func (s *ThrottleStore) Save(ctx context.Context, identityID uuid.UUID, t domain.Throttle) error {
	if identityID != s.itx.scope.IdentityID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossIdentityWrite, identityID, s.itx.scope.IdentityID())
	}
	const q = `
		INSERT INTO auth_throttle (identity_id, failures, locked_until, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (identity_id)
		DO UPDATE SET failures = EXCLUDED.failures, locked_until = EXCLUDED.locked_until, updated_at = now()`
	var lockedUntil interface{}
	if !t.LockedUntil.IsZero() {
		lockedUntil = t.LockedUntil
	}
	if _, err := s.itx.tx.Exec(ctx, q, identityID.String(), t.Failures, lockedUntil); err != nil {
		return fmt.Errorf("postgres: gravação de throttle falhou: %w", err)
	}
	return nil
}
