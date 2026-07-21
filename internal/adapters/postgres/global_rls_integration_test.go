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
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- test doubles for the global-access ports ---

type allowAuthorizer struct{}

func (allowAuthorizer) Authorize(_ context.Context, a domain.GlobalAccess) error { return a.Validate() }

type denyAuthorizer struct{}

func (denyAuthorizer) Authorize(_ context.Context, _ domain.GlobalAccess) error {
	return domain.ErrGlobalAccessDenied
}

type countingAuditor struct{ n int }

func (c *countingAuditor) Record(_ context.Context, _ domain.GlobalAccess) error { c.n++; return nil }

type failAuditor struct{}

func (failAuditor) Record(_ context.Context, _ domain.GlobalAccess) error {
	return errors.New("trilha indisponível")
}

// identityInTwoOrgs creates one identity with an active membership in two fresh
// organizations, returning the identity id.
func identityInTwoOrgs(t *testing.T, pool *pgxpool.Pool, label string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	for i, org := range []string{"g1-" + label, "g2-" + label} {
		var orgID string
		if err := pool.QueryRow(ctx,
			"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", org).Scan(&orgID); err != nil {
			t.Fatalf("org %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx,
			"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'active')",
			idn.ID.String(), orgID); err != nil {
			t.Fatalf("membership %d: %v", i, err)
		}
		t.Cleanup(func() {
			bg := context.Background()
			_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
			_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
		})
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE id = $1", idn.ID.String())
	})
	return idn.ID
}

func TestGlobalRepositoryReadsAcrossTenants(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	idnID := identityInTwoOrgs(t, pool, "read")

	audit := &countingAuditor{}
	repo := NewGlobalRepository(pool, allowAuthorizer{}, audit)
	access := domain.GlobalAccess{Principal: idnID.String(), Reason: "listar meus tenants"}

	var got []domain.Membership
	if err := repo.WithGlobalTx(ctx, access, func(tx pgx.Tx) error {
		var e error
		got, e = NewMembershipStore(tx).ListByIdentity(ctx, idnID)
		return e
	}); err != nil {
		t.Fatalf("WithGlobalTx: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("leitura global deveria ver 2 memberships (cross-tenant), viu %d", len(got))
	}
	if audit.n != 1 {
		t.Errorf("acesso cross-tenant deveria ter sido auditado 1 vez, foi %d", audit.n)
	}
}

func TestGlobalRepositoryDeniedWithoutAuthorization(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	idnID := identityInTwoOrgs(t, pool, "deny")

	audit := &countingAuditor{}
	repo := NewGlobalRepository(pool, denyAuthorizer{}, audit)
	access := domain.GlobalAccess{Principal: "x", Reason: "tentativa"}

	ran := false
	err := repo.WithGlobalTx(ctx, access, func(tx pgx.Tx) error { ran = true; return nil })
	if !errors.Is(err, domain.ErrGlobalAccessDenied) {
		t.Errorf("erro = %v, quer ErrGlobalAccessDenied", err)
	}
	if ran {
		t.Error("a transação não deveria rodar sob negação de autorização")
	}
	if audit.n != 0 {
		t.Error("acesso negado não deveria ter sido auditado como ocorrido")
	}
	_ = idnID
}

func TestGlobalRepositoryDeniedWhenAuditFails(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	repo := NewGlobalRepository(pool, allowAuthorizer{}, failAuditor{})
	access := domain.GlobalAccess{Principal: "x", Reason: "tentativa"}

	ran := false
	err := repo.WithGlobalTx(ctx, access, func(tx pgx.Tx) error { ran = true; return nil })
	if !errors.Is(err, domain.ErrGlobalAuditUnavailable) {
		t.Errorf("erro = %v, quer ErrGlobalAuditUnavailable (I-5.4)", err)
	}
	if ran {
		t.Error("a transação não deveria rodar quando a auditoria falha (fail-closed)")
	}
}

func TestGlobalRepositoryRejectsReasonlessAccess(t *testing.T) {
	pool := setupTenantPool(t)
	repo := NewGlobalRepository(pool, allowAuthorizer{}, &countingAuditor{})
	err := repo.WithGlobalTx(context.Background(), domain.GlobalAccess{Principal: "x"}, func(tx pgx.Tx) error { return nil })
	if !errors.Is(err, domain.ErrGlobalAccessDenied) {
		t.Errorf("acesso sem motivo: erro = %v, quer ErrGlobalAccessDenied", err)
	}
}

// --- Barreira 2 (RLS), tested as a non-superuser role ---

const rlsTestRole = "archguard_rls_test"

// grantRLSRole creates a non-superuser, NOBYPASSRLS role and grants it read/write
// on the RLS-enabled tables, so the test can exercise the policies (a superuser
// would bypass RLS entirely).
func grantRLSRole(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + rlsTestRole + `') THEN
				CREATE ROLE ` + rlsTestRole + ` NOLOGIN NOBYPASSRLS;
			END IF;
		END $$;`,
		`GRANT USAGE ON SCHEMA public TO ` + rlsTestRole,
		`GRANT SELECT, INSERT, UPDATE ON membership TO ` + rlsTestRole,
		`GRANT SELECT, INSERT ON role_assignment TO ` + rlsTestRole,
		`GRANT SELECT, INSERT, UPDATE ON auth_session TO ` + rlsTestRole,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("grant RLS role (%s): %v", s, err)
		}
	}
}

func TestRLSIsolatesTenantsForAppRole(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	grantRLSRole(t, pool)

	a := makeTenant(t, pool, "rls-x-a")
	b := makeTenant(t, pool, "rls-x-b")

	// Seed a role_assignment in each tenant AS SUPERUSER (bypasses RLS on write).
	for _, fx := range []tenantFixture{a, b} {
		ra, _ := domain.NewRoleAssignment(fx.orgID, fx.roleID, fx.membershipID)
		if _, err := pool.Exec(ctx,
			"INSERT INTO role_assignment (id, organization_id, role_id, membership_id) VALUES ($1,$2,$3,$4)",
			ra.ID.String(), fx.orgID.String(), fx.roleID.String(), fx.membershipID.String()); err != nil {
			t.Fatalf("seed role_assignment: %v", err)
		}
	}

	// Acquire a connection and act as the non-superuser role for the rest.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	// Under tenant A, count role_assignments visible for A's and B's membership.
	countAs := func(t *testing.T, orgID uuid.UUID, membershipID uuid.UUID, globalRead bool) int {
		t.Helper()
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+rlsTestRole); err != nil {
			t.Fatalf("set role: %v", err)
		}
		if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)", domain.RLSOrgSettingName, orgID.String()); err != nil {
			t.Fatalf("set org: %v", err)
		}
		if globalRead {
			if _, err := tx.Exec(ctx, "SELECT set_config($1,'on',true)", domain.RLSGlobalReadSettingName); err != nil {
				t.Fatalf("set global: %v", err)
			}
		}
		var n int
		if err := tx.QueryRow(ctx,
			"SELECT count(*) FROM role_assignment WHERE membership_id = $1", membershipID.String()).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	// Barreira 2: as tenant A, A's own row is visible, B's is NOT — even though the
	// query has no application-side organization_id predicate.
	if got := countAs(t, a.orgID, a.membershipID, false); got != 1 {
		t.Errorf("tenant A vendo o próprio vínculo: %d, quer 1", got)
	}
	if got := countAs(t, a.orgID, b.membershipID, false); got != 0 {
		t.Errorf("tenant A vendo vínculo de B: %d, quer 0 (RLS furada)", got)
	}
	// Global read mode: B's row becomes visible (the GlobalRepository path).
	if got := countAs(t, a.orgID, b.membershipID, true); got != 1 {
		t.Errorf("leitura global deveria ver o vínculo de B: %d, quer 1", got)
	}
}

func TestRLSWithCheckBlocksCrossTenantWrite(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	grantRLSRole(t, pool)

	a := makeTenant(t, pool, "rls-w-a")
	b := makeTenant(t, pool, "rls-w-b")

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+rlsTestRole); err != nil {
		t.Fatalf("set role: %v", err)
	}
	// Active tenant is A; attempt to insert a row for tenant B.
	if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)", domain.RLSOrgSettingName, a.orgID.String()); err != nil {
		t.Fatalf("set org: %v", err)
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO role_assignment (id, organization_id, role_id, membership_id) VALUES (gen_random_uuid(), $1, $2, $3)",
		b.orgID.String(), b.roleID.String(), b.membershipID.String())
	if err == nil {
		t.Error("WITH CHECK deveria bloquear escrita para outro tenant sob a RLS")
	}
}
