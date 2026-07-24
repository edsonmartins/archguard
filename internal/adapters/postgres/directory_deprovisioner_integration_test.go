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

// Desativação no diretório em UMA execução: o membership é suspenso, as sessões do
// tenant encerradas e nenhum registro histórico removido (spec "Usuário desativado
// no diretório"). Re-executar é no-op idempotente.
func TestDirectoryDeprovisionerSuspends(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "deprov")

	// Sessão ativa vinculada ao membership de A.
	at := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	sessID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, auth_methods)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5,ARRAY['password','totp']::text[])`,
		sessID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), at); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}

	repo := NewTenantRepository(pool, fx.tenantScopeA)
	deprov := NewDirectoryDeprovisioner(repo, nil)

	m, sessions, err := deprov.SuspendForDeactivation(ctx, fx.memA.ID)
	if err != nil {
		t.Fatalf("SuspendForDeactivation: %v", err)
	}
	if m.Status != domain.MembershipSuspended {
		t.Fatalf("membership deveria estar suspenso, veio %s", m.Status)
	}
	if sessions != 1 {
		t.Fatalf("deveria ter encerrado 1 sessão, veio %d", sessions)
	}

	// O membership permanece (não foi deletado) e está suspenso no banco.
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM membership WHERE id = $1", fx.memA.ID.String()).Scan(&status); err != nil {
		t.Fatalf("membership deveria continuar existindo: %v", err)
	}
	if status != "suspended" {
		t.Fatalf("status persistido deveria ser suspended, veio %s", status)
	}

	// Idempotente: re-executar não suspende de novo nem encerra sessões.
	m2, sessions2, err := deprov.SuspendForDeactivation(ctx, fx.memA.ID)
	if err != nil {
		t.Fatalf("segunda execução: %v", err)
	}
	if sessions2 != 0 || m2.Status != domain.MembershipSuspended {
		t.Fatalf("segunda execução deveria ser no-op: sessions=%d status=%s", sessions2, m2.Status)
	}
}
