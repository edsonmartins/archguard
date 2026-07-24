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

// T-019 / spec "Usuário desativado no diretório": em UMA execução de reconciliação,
// um registro de sync com Active=false suspende o membership correspondente e
// encerra as sessões do tenant — sem remover nada.
func TestDirectoryDeactivationSuspendsInOnePass(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	org := makeTenant(t, pool, "deact-e2e")

	// Provisiona a pessoa (identidade + membership ativo) via o caminho de dedup.
	email := "sai@empresa.com"
	hash, _ := cust.HashEmail(email)
	idStr, err := NewDirectoryProvisioner(pool, cust).ProvisionUser(ctx, org.orgID,
		domain.DirectorySyncRecord{Email: email, Active: true})
	if err != nil {
		t.Fatalf("provisão: %v", err)
	}
	identityID := uuid.MustParse(idStr)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM auth_session WHERE identity_id = $1", identityID.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id = $1", identityID.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", hash)
	})

	// Sessão ativa do tenant para essa identidade.
	var membershipID uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT id FROM membership WHERE identity_id = $1 AND organization_id = $2",
		identityID.String(), org.orgID.String()).Scan(&membershipID); err != nil {
		t.Fatalf("membership: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time, auth_methods)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5,ARRAY['password','totp']::text[])`,
		uuid.New().String(), identityID.String(), membershipID.String(), org.orgID.String(),
		time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}

	// UMA execução de reconciliação: o diretório desativou o usuário (Active=false).
	rec := domain.DirectorySyncRecord{Email: email, Active: false}
	// A decisão de domínio confirma que a suspensão é necessária...
	if !rec.RequiresSuspension(domain.MembershipActive) {
		t.Fatalf("registro inativo sobre membership ativo deveria exigir suspensão")
	}
	// ...e o desprovisionador a aplica (suspende + encerra sessões) em uma tx.
	repo := NewTenantRepository(pool, org.scope)
	m, sessions, err := NewDirectoryDeprovisioner(repo, nil).SuspendForDeactivation(ctx, membershipID)
	if err != nil {
		t.Fatalf("SuspendForDeactivation: %v", err)
	}
	if m.Status != domain.MembershipSuspended {
		t.Fatalf("membership deveria estar suspenso, veio %s", m.Status)
	}
	if sessions != 1 {
		t.Fatalf("deveria ter encerrado 1 sessão, veio %d", sessions)
	}

	// Nada foi removido: o membership e a identidade continuam existindo.
	var memStatus, idExists int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM membership WHERE id = $1 AND status = 'suspended'", membershipID.String()).Scan(&memStatus)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE id = $1", identityID.String()).Scan(&idExists)
	if memStatus != 1 || idExists != 1 {
		t.Fatalf("nenhum registro histórico deveria ser removido (mem=%d id=%d)", memStatus, idExists)
	}
}
