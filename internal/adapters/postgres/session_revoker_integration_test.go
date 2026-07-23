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
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// A revogação local do logout encerra a sessão E revoga os refresh tokens
// derivados, atomicamente (cenário "Logout no ArchGuard").
func TestSessionRevokerRevokesSessionAndRefresh(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "sessrev")

	at := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	sessID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5)`,
		sessID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), at); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}

	repo := NewTenantRepository(pool, fx.tenantScopeA)
	_, h, _ := domain.NewRefreshSecret()
	rt, _ := domain.NewRefreshFamily(sessID, fx.orgA, h, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewRefreshTokenStore(ttx).Create(ctx, rt)
	}); err != nil {
		t.Fatalf("cria refresh: %v", err)
	}

	if err := NewSessionRevoker(repo).RevokeSession(ctx, sessID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	var sessStatus, rtStatus string
	_ = pool.QueryRow(ctx, "SELECT status FROM auth_session WHERE id = $1", sessID.String()).Scan(&sessStatus)
	_ = pool.QueryRow(ctx, "SELECT status FROM refresh_token WHERE id = $1", rt.ID.String()).Scan(&rtStatus)
	if sessStatus != "revoked" {
		t.Fatalf("a sessão deveria estar revogada, veio %s", sessStatus)
	}
	if rtStatus != "revoked" {
		t.Fatalf("o refresh derivado deveria estar revogado, veio %s", rtStatus)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM refresh_token WHERE session_id = $1", sessID.String())
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE id = $1", sessID.String())
	})
}
