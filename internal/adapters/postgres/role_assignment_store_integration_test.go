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
	"os"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// roleAssignFixture holds the real ids a role assignment needs to reference.
type roleAssignFixture struct {
	pool         *pgxpool.Pool
	orgID        uuid.UUID
	roleID       uuid.UUID
	membershipID uuid.UUID
}

func setupRoleAssignment(t *testing.T) (*RoleAssignmentStore, roleAssignFixture) {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — pulando teste de integração do RoleAssignmentStore")
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

	suffix := t.Name()
	var fx roleAssignFixture
	fx.pool = pool
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-"+suffix).Scan(&fx.orgID); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO role (owner, name) VALUES ('it', $1) RETURNING id", "role-"+suffix).Scan(&fx.roleID); err != nil {
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
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM role_assignment WHERE organization_id = $1", fx.orgID.String())
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", fx.orgID.String())
		_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", idn.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM role WHERE id = $1", fx.roleID.String())
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", fx.orgID.String())
	})
	return NewRoleAssignmentStore(pool), fx
}

func TestRoleAssignmentCreateAndList(t *testing.T) {
	store, fx := setupRoleAssignment(t)
	ctx := context.Background()

	ra, err := domain.NewRoleAssignment(fx.orgID, fx.roleID, fx.membershipID)
	if err != nil {
		t.Fatalf("NewRoleAssignment: %v", err)
	}
	if err := store.Create(ctx, ra); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := store.ListByMembership(ctx, fx.membershipID)
	if err != nil {
		t.Fatalf("ListByMembership: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("esperava 1 vínculo, veio %d", len(got))
	}
	if got[0].RoleID != fx.roleID || got[0].MembershipID != fx.membershipID || got[0].OrganizationID != fx.orgID {
		t.Errorf("vínculo recomposto errado: %+v", got[0])
	}
}

func TestRoleAssignmentUniquePair(t *testing.T) {
	store, fx := setupRoleAssignment(t)
	ctx := context.Background()
	a, _ := domain.NewRoleAssignment(fx.orgID, fx.roleID, fx.membershipID)
	if err := store.Create(ctx, a); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	// Mesmo (role_id, membership_id) de novo deve violar a UNIQUE.
	b, _ := domain.NewRoleAssignment(fx.orgID, fx.roleID, fx.membershipID)
	if err := store.Create(ctx, b); err == nil {
		t.Error("par (role_id, membership_id) duplicado deveria violar a UNIQUE")
	}
}

func TestRoleAssignmentFKsEnforced(t *testing.T) {
	store, fx := setupRoleAssignment(t)
	ctx := context.Background()

	// role_id inexistente.
	bad1 := domain.RoleAssignment{ID: mustV7(t), OrganizationID: fx.orgID, RoleID: mustV7(t), MembershipID: fx.membershipID}
	if err := store.Create(ctx, bad1); err == nil {
		t.Error("role_id inexistente deveria violar a FK")
	}
	// membership_id inexistente.
	bad2 := domain.RoleAssignment{ID: mustV7(t), OrganizationID: fx.orgID, RoleID: fx.roleID, MembershipID: mustV7(t)}
	if err := store.Create(ctx, bad2); err == nil {
		t.Error("membership_id inexistente deveria violar a FK")
	}
	// organization_id inexistente.
	bad3 := domain.RoleAssignment{ID: mustV7(t), OrganizationID: mustV7(t), RoleID: fx.roleID, MembershipID: fx.membershipID}
	if err := store.Create(ctx, bad3); err == nil {
		t.Error("organization_id inexistente deveria violar a FK")
	}
}

func mustV7(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
