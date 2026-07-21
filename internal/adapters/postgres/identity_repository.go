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
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/jackc/pgx/v5"
)

// IdentityRepository is the identity-axis mirror of TenantRepository: the entry
// point for the LOGIN FLOW's access to auth_session (create the session, select
// the tenant, switch, read/revoke own sessions). It cannot exist without an
// IdentityScope, and each operation runs in one transaction in which the
// identity is pinned as a SET LOCAL setting — which the auth_session RLS policy
// (migration 0013) reads. Both barriers, on the identity axis, armed by
// construction. auth_session rows pending tenant selection have no organization,
// so this is the ONLY scoped path that reaches them.
type IdentityRepository struct {
	db    Beginner
	scope domain.IdentityScope
}

// NewIdentityRepository binds a connection source to an identity scope. There is
// no constructor that omits the scope.
func NewIdentityRepository(db Beginner, scope domain.IdentityScope) *IdentityRepository {
	return &IdentityRepository{db: db, scope: scope}
}

// Scope returns the identity this repository is bound to.
func (r *IdentityRepository) Scope() domain.IdentityScope { return r.scope }

// IdentityTx is an open transaction already pinned to an identity: the SET
// LOCAL setting is applied, and the scope backs the explicit identity_id
// predicates the stores add (defense-in-depth that holds even with RLS off).
type IdentityTx struct {
	tx    pgx.Tx
	scope domain.IdentityScope
}

// Tx exposes the underlying transaction for stores built on it.
func (t *IdentityTx) Tx() pgx.Tx { return t.tx }

// Scope returns the identity pinned on this transaction.
func (t *IdentityTx) Scope() domain.IdentityScope { return t.scope }

// WithIdentityTx runs fn inside one transaction with the identity pinned as a
// SET LOCAL setting (transaction-scoped, never leaks to the next borrower of a
// pooled connection). Committed if fn returns nil, rolled back otherwise.
func (r *IdentityRepository) WithIdentityTx(ctx context.Context, fn func(*IdentityTx) error) error {
	return WithTx(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)",
			domain.RLSIdentitySettingName, r.scope.IdentityID().String()); err != nil {
			return fmt.Errorf("postgres: fixação da identidade na sessão falhou: %w", err)
		}
		return fn(&IdentityTx{tx: tx, scope: r.scope})
	})
}
