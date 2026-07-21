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

// TenantRepository is the entry point for tenant-scoped data access — Barreira 1
// of the two-barrier isolation (RFC-0002 §4). It cannot exist without a
// TenantScope, so no access it mediates is ever unscoped. Each operation runs in
// its own transaction (RFC-0002 §5) in which the tenant is pinned as a SET LOCAL
// session setting, which the Row-Level Security policies (Barreira 2, T-010)
// read — so both barriers are armed by construction.
type TenantRepository struct {
	db    Beginner
	scope domain.TenantScope
}

// NewTenantRepository binds a connection source to a tenant scope. There is no
// constructor that omits the scope: Barreira 1 lives in the type.
func NewTenantRepository(db Beginner, scope domain.TenantScope) *TenantRepository {
	return &TenantRepository{db: db, scope: scope}
}

// Scope returns the tenant this repository is bound to.
func (r *TenantRepository) Scope() domain.TenantScope { return r.scope }

// TenantTx is an open transaction already pinned to a tenant: the SET LOCAL
// session setting is applied, and the scope is available for the explicit
// organization_id predicates that stores add (defense-in-depth that holds even
// with RLS disabled). Multiple stores share one TenantTx to compose a single
// business-operation transaction (RFC-0002 §5).
type TenantTx struct {
	tx    pgx.Tx
	scope domain.TenantScope
}

// Tx exposes the underlying transaction for stores built on it.
func (t *TenantTx) Tx() pgx.Tx { return t.tx }

// Scope returns the tenant pinned on this transaction.
func (t *TenantTx) Scope() domain.TenantScope { return t.scope }

// WithTenantTx runs fn inside one transaction with the active tenant pinned as a
// SET LOCAL setting (transaction-scoped, so it can never leak to the next
// borrower of a pooled connection). fn receives a TenantTx to build stores on.
// Committed if fn returns nil, rolled back otherwise (via WithTx).
func (r *TenantRepository) WithTenantTx(ctx context.Context, fn func(*TenantTx) error) error {
	return WithTx(ctx, r.db, func(tx pgx.Tx) error {
		// SET LOCAL via set_config(name, value, is_local=true): scoped to this
		// transaction, released on commit/rollback. This is the value the RLS
		// policies read via current_setting(domain.RLSOrgSettingName).
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)",
			domain.RLSOrgSettingName, r.scope.OrganizationID().String()); err != nil {
			return fmt.Errorf("postgres: fixação do tenant na sessão falhou: %w", err)
		}
		return fn(&TenantTx{tx: tx, scope: r.scope})
	})
}
