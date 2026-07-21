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

// GlobalRepository is the DISTINCT, explicit path for cross-tenant reads
// (RFC-0002 §4): global reports, "all memberships of this identity". It is
// deliberately not a TenantScope with a wildcard — crossing tenants is a separate
// type so it can never happen by accident. Every access is authorized (INV-6
// fail-closed) and audited (I-5.4 audit-or-deny) before the transaction runs with
// the global-read flag the RLS policies honor (Barreira 2, T-010).
type GlobalRepository struct {
	db    Beginner
	authz domain.GlobalAuthorizer
	audit domain.AccessAuditor
}

// NewGlobalRepository wires the connection source with the authorization and
// audit ports. Neither is optional: there is no cross-tenant access without a
// decision and a record.
func NewGlobalRepository(db Beginner, authz domain.GlobalAuthorizer, audit domain.AccessAuditor) *GlobalRepository {
	return &GlobalRepository{db: db, authz: authz, audit: audit}
}

// WithGlobalTx authorizes and audits the access, then runs fn in one transaction
// with the global-read flag set (SET LOCAL). Order is load-bearing:
//  1. the access must be well-formed (principal + reason);
//  2. authorization must allow it — any authorizer error denies (INV-6);
//  3. the access must be recorded durably — an audit failure denies (I-5.4);
//  4. only then does the read run, cross-tenant, under the honored flag.
func (r *GlobalRepository) WithGlobalTx(ctx context.Context, access domain.GlobalAccess, fn func(pgx.Tx) error) error {
	if err := access.Validate(); err != nil {
		return err
	}
	if err := r.authz.Authorize(ctx, access); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGlobalAccessDenied, err)
	}
	if err := r.audit.Record(ctx, access); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrGlobalAuditUnavailable, err)
	}
	return WithTx(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, 'on', true)", domain.RLSGlobalReadSettingName); err != nil {
			return fmt.Errorf("postgres: ativação da leitura global falhou: %w", err)
		}
		return fn(tx)
	})
}
