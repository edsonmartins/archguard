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
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccessAuditor is the DURABLE cross-tenant access auditor (pacote 007, ADR-0022).
// It records every global access (login/console resolving own memberships, global
// reports) in the append-only global_access_audit table (migration 0035) BEFORE the
// access runs — the GlobalRepository denies on a Record failure (I-5.4). It replaces
// the provisional in-memory auditor in pilot/production.
type AccessAuditor struct {
	pool *pgxpool.Pool
}

// NewAccessAuditor builds the durable auditor over the pool.
func NewAccessAuditor(pool *pgxpool.Pool) *AccessAuditor {
	return &AccessAuditor{pool: pool}
}

// Record durably appends the access. It validates first (an ill-formed access is
// not recorded as if real) and inserts principal, reason and scope. A write failure
// is returned so the caller denies the cross-tenant read (fail-closed, I-5.4).
func (a *AccessAuditor) Record(ctx context.Context, access domain.GlobalAccess) error {
	if err := access.Validate(); err != nil {
		return err
	}
	const q = `INSERT INTO global_access_audit (principal, reason, scope) VALUES ($1, $2, $3)`
	if _, err := a.pool.Exec(ctx, q, access.Principal, access.Reason, access.Scope.String()); err != nil {
		return fmt.Errorf("postgres: registro de acesso global falhou: %w", err)
	}
	return nil
}

var _ domain.AccessAuditor = (*AccessAuditor)(nil)
