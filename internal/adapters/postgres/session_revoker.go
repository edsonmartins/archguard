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
)

// SessionRevoker implements domain.SessionRevoker: it revokes an auth_session AND
// its derived refresh-token families in ONE transaction (the fail-closed local
// leg of back-channel logout, T-009). It is tenant-scoped; the org is the
// session's tenant.
type SessionRevoker struct {
	repo *TenantRepository
}

// NewSessionRevoker builds the revoker over the tenant repository.
func NewSessionRevoker(repo *TenantRepository) *SessionRevoker {
	return &SessionRevoker{repo: repo}
}

// RevokeSession revokes the session and every refresh token derived from it,
// atomically. Idempotent — revoking an already-revoked session/tokens is a no-op.
func (r *SessionRevoker) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	return r.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		// Revoke the session row (tenant-scoped: only this org's session).
		tag, err := ttx.Tx().Exec(ctx,
			`UPDATE auth_session
			 SET status = 'revoked', revoked_at = COALESCE(revoked_at, now()), updated_at = now()
			 WHERE id = $1 AND organization_id = $2`,
			sessionID.String(), r.repo.Scope().OrganizationID().String())
		if err != nil {
			return fmt.Errorf("postgres: revogação de sessão no logout falhou: %w", err)
		}
		_ = tag // a session already gone (revoked/absent) is fine — idempotent.
		// Revoke the derived refresh-token families.
		if _, err := NewRefreshTokenStore(ttx).RevokeBySession(ctx, sessionID); err != nil {
			return err
		}
		return nil
	})
}

// ensure SessionRevoker satisfies the domain port.
var _ domain.SessionRevoker = (*SessionRevoker)(nil)
