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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionLivenessChecker implements domain.SessionLiveness over the pool: a
// session is LIVE when its auth_session row is active in the given organization.
// It reads under the org's tenant context (so RLS admits exactly that org's
// session). It is the introspection path that makes a revoked session read as
// inactive for a component without back-channel logout (T-010).
type SessionLivenessChecker struct {
	pool *pgxpool.Pool
}

// NewSessionLivenessChecker builds the checker over the connection pool.
func NewSessionLivenessChecker(pool *pgxpool.Pool) *SessionLivenessChecker {
	return &SessionLivenessChecker{pool: pool}
}

// Live reports whether session sid is active in organizationID. Malformed ids or
// a query failure return an error — the caller (introspection) fails closed to
// active:false, never treating an unverifiable session as live.
func (c *SessionLivenessChecker) Live(ctx context.Context, organizationID, sid string) (bool, error) {
	orgUUID, err := uuid.Parse(organizationID)
	if err != nil {
		return false, fmt.Errorf("postgres: org inválida %q: %w", organizationID, err)
	}
	sessUUID, err := uuid.Parse(sid)
	if err != nil {
		return false, fmt.Errorf("postgres: sid inválido %q: %w", sid, err)
	}
	scope, err := domain.NewTenantScope(orgUUID)
	if err != nil {
		return false, err
	}
	var live bool
	err = NewTenantRepository(c.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return ttx.Tx().QueryRow(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM auth_session
				WHERE id = $1 AND organization_id = $2 AND status = 'active')`,
			sessUUID.String(), orgUUID.String()).Scan(&live)
	})
	if err != nil {
		return false, fmt.Errorf("postgres: checagem de liveness falhou: %w", err)
	}
	return live, nil
}

// ensure it satisfies the domain port.
var _ domain.SessionLiveness = (*SessionLivenessChecker)(nil)
