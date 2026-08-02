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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// TestMembershipLifecycleProjection: ao SUSPENDER um membership, suas tuplas de acesso
// (owner + operator) são enfileiradas como DELETE na mesma tx; ao REATIVAR, como WRITE
// (M4 Fase E, T-030) — quem sai do tenant não mantém acesso no grafo.
func TestMembershipLifecycleProjection(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-mlc-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	m, err := domain.NewMembership(idn.ID, orgID)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	ownedAsset, otherAsset := uuid.New(), uuid.New()
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset_access_assignment WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})
	// membership ativo + um ativo que ele possui + uma atribuição operator sobre outro ativo.
	if _, err := pool.Exec(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,$4)",
		m.ID.String(), m.IdentityID.String(), orgID.String(), string(m.Status)); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO asset (id, organization_id, kind, name, owner_membership_id) VALUES ($1,$2,'host','a',$3)",
		ownedAsset.String(), orgID.String(), m.ID.String()); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO asset_access_assignment (id, organization_id, subject_type, subject_id, relation, object_type, object_id) VALUES ($1,$2,'membership',$3,'operator','asset',$4)",
		uuid.New().String(), orgID.String(), m.ID.String(), otherAsset.String()); err != nil {
		t.Fatalf("insert assignment: %v", err)
	}

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	repo := NewTenantRepository(pool, scope)
	subjectRef := domain.Qualify(orgID, domain.TypeMembership, m.ID.String())

	countOutbox := func(op string) int {
		var n int
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id=$1 AND tuple_user=$2 AND op=$3",
			orgID, subjectRef, op).Scan(&n); err != nil {
			t.Fatalf("count outbox %s: %v", op, err)
		}
		return n
	}

	// Suspende → 2 DELETE (owner + operator) do membership.
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewTenantMembershipStore(ttx).SaveSuspension(ctx, m)
	}); err != nil {
		t.Fatalf("SaveSuspension: %v", err)
	}
	if got := countOutbox("delete"); got != 2 {
		t.Errorf("após suspensão esperava 2 DELETE (owner+operator), veio %d", got)
	}

	// Reativa → 2 WRITE (restaura owner + operator).
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewTenantMembershipStore(ttx).SaveReactivation(ctx, m)
	}); err != nil {
		t.Fatalf("SaveReactivation: %v", err)
	}
	if got := countOutbox("write"); got != 2 {
		t.Errorf("após reativação esperava 2 WRITE (owner+operator), veio %d", got)
	}
}

// TestRevokeMembershipCascadesGrants (T-030b): revogar um membership cascade-revoga suas
// concessões ativas — o grant vira 'revoked' e o has_active_grant é apagado do grafo (via
// enqueueGrantProjection), na mesma tx. Quem saiu não mantém concessão privilegiada.
func TestRevokeMembershipCascadesGrants(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-casc-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	m, err := domain.NewMembership(idn.ID, orgID)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	grantID, assetID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM privileged_grant WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})
	if _, err := pool.Exec(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,'active')",
		m.ID.String(), m.IdentityID.String(), orgID.String()); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	// concessão ATIVA sobre um ativo, do membership.
	if _, err := pool.Exec(ctx,
		`INSERT INTO privileged_grant
		 (id, organization_id, subject_membership_id, target_type, target_id, target_scope,
		  origin, status, required_approvals, not_before, expires_at, justification, incident_ref)
		 VALUES ($1,$2,$3,'asset',$4,'admin','normal','active',0, now()-interval '1 hour', now()+interval '1 hour','j','INC-1')`,
		grantID.String(), orgID.String(), m.ID.String(), assetID.String()); err != nil {
		t.Fatalf("insert grant: %v", err)
	}

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	if _, _, err := NewMembershipRevoker(NewTenantRepository(pool, scope), nil).RevokeMembership(ctx, m.ID); err != nil {
		t.Fatalf("RevokeMembership: %v", err)
	}

	// grant virou revoked.
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM privileged_grant WHERE id=$1", grantID.String()).Scan(&status); err != nil {
		t.Fatalf("status grant: %v", err)
	}
	if status != "revoked" {
		t.Errorf("grant status = %q, esperado revoked", status)
	}
	// has_active_grant apagado no grafo (delete enfileirado).
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id=$1 AND tuple_relation='has_active_grant' AND op='delete'",
		orgID).Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if n == 0 {
		t.Error("esperava DELETE de has_active_grant no outbox após cascade")
	}
}
