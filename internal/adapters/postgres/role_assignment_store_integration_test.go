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
	"errors"
	"os"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantFixture is one tenant with a role and a membership to bind.
type tenantFixture struct {
	orgID        uuid.UUID
	roleID       uuid.UUID
	membershipID uuid.UUID
	scope        domain.TenantScope
}

func makeTenant(t *testing.T, pool *pgxpool.Pool, label string) tenantFixture {
	t.Helper()
	ctx := context.Background()
	var fx tenantFixture
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-"+label).Scan(&fx.orgID); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO role (owner, name) VALUES ('it', $1) RETURNING id", "role-"+label).Scan(&fx.roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'active') RETURNING id",
		idn.ID.String(), fx.orgID.String()).Scan(&fx.membershipID); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	scope, err := domain.NewTenantScope(fx.orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	fx.scope = scope
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM role_assignment WHERE organization_id = $1", fx.orgID.String())
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", fx.orgID.String())
		_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", idn.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM role WHERE id = $1", fx.roleID.String())
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", fx.orgID.String())
	})
	return fx
}

func setupTenantPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — pulando teste de integração tenant-scoped")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	seedLegacyTables(t, pool)
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return pool
}

// assign is a small helper: run a create within a tenant transaction.
func assign(ctx context.Context, repo *TenantRepository, ra domain.RoleAssignment) error {
	return repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewRoleAssignmentStore(ttx).Create(ctx, ra)
	})
}

func TestTenantRoleAssignmentCreateAndList(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "cl-a")
	repo := NewTenantRepository(pool, a.scope)

	ra, err := domain.NewRoleAssignment(a.orgID, a.roleID, a.membershipID)
	if err != nil {
		t.Fatalf("NewRoleAssignment: %v", err)
	}
	if err := assign(ctx, repo, ra); err != nil {
		t.Fatalf("assign: %v", err)
	}

	var got []domain.RoleAssignment
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		got, e = NewRoleAssignmentStore(ttx).ListByMembership(ctx, a.membershipID)
		return e
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].RoleID != a.roleID || got[0].OrganizationID != a.orgID {
		t.Fatalf("vínculo errado: %+v", got)
	}
}

// TestTenantRoleAssignmentRejectsCrossTenantWrite proves Barreira 1 on writes: a
// store scoped to tenant A cannot create a row belonging to tenant B.
func TestTenantRoleAssignmentRejectsCrossTenantWrite(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "xw-a")
	b := makeTenant(t, pool, "xw-b")
	repoA := NewTenantRepository(pool, a.scope)

	// A binding that belongs to tenant B, attempted through tenant A's repo.
	raB, err := domain.NewRoleAssignment(b.orgID, b.roleID, b.membershipID)
	if err != nil {
		t.Fatalf("NewRoleAssignment: %v", err)
	}
	if err := assign(ctx, repoA, raB); !errors.Is(err, ErrCrossTenantWrite) {
		t.Errorf("escrita cross-tenant: erro = %v, quer ErrCrossTenantWrite", err)
	}
}

// TestTenantRoleAssignmentReadIsolation proves Barreira 1 on reads (RLS off): a
// store scoped to tenant A never returns tenant B's rows, even when querying by
// B's membership id.
func TestTenantRoleAssignmentReadIsolation(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "ri-a")
	b := makeTenant(t, pool, "ri-b")

	// Create a real binding in tenant B (through B's own repo).
	repoB := NewTenantRepository(pool, b.scope)
	raB, _ := domain.NewRoleAssignment(b.orgID, b.roleID, b.membershipID)
	if err := assign(ctx, repoB, raB); err != nil {
		t.Fatalf("assign B: %v", err)
	}

	// Tenant A's repo, asked for B's membership, must see nothing.
	repoA := NewTenantRepository(pool, a.scope)
	var leaked []domain.RoleAssignment
	if err := repoA.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		leaked, e = NewRoleAssignmentStore(ttx).ListByMembership(ctx, b.membershipID)
		return e
	}); err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("travessia: repo de A retornou %d vínculo(s) de B (Barreira 1 furada)", len(leaked))
	}
	// Sanity: B's own repo does see it.
	var own []domain.RoleAssignment
	_ = repoB.WithTenantTx(ctx, func(ttx *TenantTx) error {
		own, _ = NewRoleAssignmentStore(ttx).ListByMembership(ctx, b.membershipID)
		return nil
	})
	if len(own) != 1 {
		t.Errorf("repo de B deveria ver o próprio vínculo, viu %d", len(own))
	}
}

// TestTenantSetsRLSSessionVar proves the tenant transaction pins the tenant in
// the SET LOCAL setting the RLS policies (T-010) will read.
func TestTenantSetsRLSSessionVar(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "rls-a")
	repo := NewTenantRepository(pool, a.scope)

	var seen string
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return ttx.Tx().QueryRow(ctx,
			"SELECT current_setting($1, true)", domain.RLSOrgSettingName).Scan(&seen)
	}); err != nil {
		t.Fatalf("current_setting: %v", err)
	}
	if seen != a.orgID.String() {
		t.Errorf("session var = %q, quer %s", seen, a.orgID)
	}
}

func TestTenantRoleAssignmentUniquePair(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "uq-a")
	repo := NewTenantRepository(pool, a.scope)

	ra1, _ := domain.NewRoleAssignment(a.orgID, a.roleID, a.membershipID)
	if err := assign(ctx, repo, ra1); err != nil {
		t.Fatalf("assign 1: %v", err)
	}
	ra2, _ := domain.NewRoleAssignment(a.orgID, a.roleID, a.membershipID)
	if err := assign(ctx, repo, ra2); err == nil {
		t.Error("par (role_id, membership_id) duplicado deveria violar a UNIQUE")
	}
}
