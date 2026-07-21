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

package invariants

// INV-5 / I-6.3 — travessia entre tenants é impossível POR CONSTRUÇÃO, provada
// em duas barreiras INDEPENDENTES (RFC-0002 §4, T-017; gate do pacote 002:
// travessia verde com RLS ligada E desligada):
//
//   - Barreira 1 (aplicação), ISOLADA: os testes rodam como SUPERUSUÁRIO, que
//     ignora RLS por completo — ou seja, com a Barreira 2 efetivamente
//     DESLIGADA. Todo isolamento observado vem dos predicados explícitos e dos
//     construtores com escopo obrigatório dos repositórios.
//   - Barreira 2 (RLS), ISOLADA: os testes rodam como papel NÃO-superusuário
//     sem BYPASSRLS, executando SQL cru SEM NENHUM predicado de aplicação —
//     ou seja, com a Barreira 1 deliberadamente contornada. Todo isolamento
//     observado vem das políticas de RLS.
//
// Falha aqui quebra o build (make invariants). Exige PostgreSQL real:
// ARCHGUARD_TEST_DSN aponta o banco descartável (sem ele, o teste é pulado —
// o CI deve prover o serviço de PG para o gate valer).

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// traversalFixture: identidade X com membership, papel e sessão em A e em B —
// o alvo que uma travessia tentaria alcançar de um lado a partir do outro.
type traversalFixture struct {
	orgA, orgB   uuid.UUID
	identity     uuid.UUID
	memA, memB   uuid.UUID
	roleA        uuid.UUID
	assignA      uuid.UUID
	sessA, sessB uuid.UUID
	scopeA       domain.TenantScope
}

func setupTraversal(t *testing.T) (*pgxpool.Pool, traversalFixture) {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — travessia exige PostgreSQL real (CI deve prover)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Espelha a ordem real de boot: Sync2 cria as tabelas legadas, depois as
	// migrations as estendem e criam as novas.
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS organization (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
		`CREATE TABLE IF NOT EXISTS role (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("seed legado: %v", err)
		}
	}
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	var fx traversalFixture
	label := uuid.NewString()[:8]
	for name, dst := range map[string]*uuid.UUID{"a": &fx.orgA, "b": &fx.orgB} {
		if err := pool.QueryRow(ctx,
			"INSERT INTO organization (owner, name) VALUES ('inv5', $1) RETURNING id",
			"trav-"+name+"-"+label).Scan(dst); err != nil {
			t.Fatalf("org %s: %v", name, err)
		}
	}
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := postgres.NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	fx.identity = idn.ID
	for org, dst := range map[uuid.UUID]*uuid.UUID{fx.orgA: &fx.memA, fx.orgB: &fx.memB} {
		if err := pool.QueryRow(ctx,
			"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'active') RETURNING id",
			fx.identity.String(), org.String()).Scan(dst); err != nil {
			t.Fatalf("membership: %v", err)
		}
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO role (owner, name) VALUES ('inv5', $1) RETURNING id", "r-"+label).Scan(&fx.roleA); err != nil {
		t.Fatalf("role: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO role_assignment (id, organization_id, role_id, membership_id) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id",
		fx.orgA.String(), fx.roleA.String(), fx.memA.String()).Scan(&fx.assignA); err != nil {
		t.Fatalf("role_assignment: %v", err)
	}
	for mem, dst := range map[uuid.UUID]*uuid.UUID{fx.memA: &fx.sessA, fx.memB: &fx.sessB} {
		var org uuid.UUID
		if mem == fx.memA {
			org = fx.orgA
		} else {
			org = fx.orgB
		}
		if err := pool.QueryRow(ctx,
			`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal)
			 VALUES (gen_random_uuid(), $1, $2, $3, 'active', 'aal1') RETURNING id`,
			fx.identity.String(), mem.String(), org.String()).Scan(dst); err != nil {
			t.Fatalf("auth_session: %v", err)
		}
	}
	if fx.scopeA, err = domain.NewTenantScope(fx.orgA); err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE identity_id = $1", fx.identity.String())
		_, _ = pool.Exec(bg, "DELETE FROM role_assignment WHERE id = $1", fx.assignA.String())
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE identity_id = $1", fx.identity.String())
		_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", fx.identity.String())
		_, _ = pool.Exec(bg, "DELETE FROM role WHERE id = $1", fx.roleA.String())
		for _, org := range []uuid.UUID{fx.orgA, fx.orgB} {
			_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", org.String())
		}
	})
	return pool, fx
}

// TestINV5TraversalBarrier1 prova a Barreira 1 ISOLADA: como superusuário a
// RLS é ignorada, então só os predicados/guardas da aplicação seguram a
// travessia — e seguram: um repositório escopado em A não lê nem escreve nada
// de B, para as três tabelas novas.
func TestINV5TraversalBarrier1(t *testing.T) {
	pool, fx := setupTraversal(t)
	ctx := context.Background()
	repo := postgres.NewTenantRepository(pool, fx.scopeA)

	// LEITURA: cada store escopado em A tenta alcançar o registro de B.
	if err := repo.WithTenantTx(ctx, func(ttx *postgres.TenantTx) error {
		ras, err := postgres.NewRoleAssignmentStore(ttx).ListByMembership(ctx, fx.memB)
		if err != nil {
			return err
		}
		if len(ras) != 0 {
			t.Errorf("Barreira 1: role_assignment de B visível de A: %+v", ras)
		}
		if _, err := postgres.NewTenantMembershipStore(ttx).Get(ctx, fx.memB); !errors.Is(err, postgres.ErrMembershipNotFound) {
			t.Errorf("Barreira 1: membership de B alcançável de A (err=%v)", err)
		}
		sessions, err := postgres.NewTenantSessionStore(ttx).ListActive(ctx)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			if s.ID == fx.sessB {
				t.Errorf("Barreira 1: sessão de B listada por A")
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("travessia de leitura: %v", err)
	}

	// ESCRITA: criar dado de B a partir do repositório de A é recusado.
	ra, err := domain.NewRoleAssignment(fx.orgB, fx.roleA, fx.memB)
	if err != nil {
		t.Fatalf("NewRoleAssignment: %v", err)
	}
	err = repo.WithTenantTx(ctx, func(ttx *postgres.TenantTx) error {
		return postgres.NewRoleAssignmentStore(ttx).Create(ctx, ra)
	})
	if !errors.Is(err, postgres.ErrCrossTenantWrite) {
		t.Errorf("Barreira 1: escrita cross-tenant de role_assignment: err=%v", err)
	}
	m, err := domain.NewMembership(fx.identity, fx.orgB)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	err = repo.WithTenantTx(ctx, func(ttx *postgres.TenantTx) error {
		return postgres.NewTenantMembershipStore(ttx).Create(ctx, m)
	})
	if !errors.Is(err, postgres.ErrCrossTenantWrite) {
		t.Errorf("Barreira 1: escrita cross-tenant de membership: err=%v", err)
	}

	// Fundamento da barreira: não existe escopo vazio.
	if _, err := domain.NewTenantScope(uuid.Nil); !errors.Is(err, domain.ErrNoTenant) {
		t.Errorf("Barreira 1: escopo nulo deveria ser recusado: %v", err)
	}
}

const inv5Role = "archguard_inv5_traversal"

// TestINV5TraversalBarrier2 prova a Barreira 2 ISOLADA: como papel
// não-superusuário (NOBYPASSRLS), SQL cru SEM predicado de aplicação só
// enxerga/escreve o que a política de RLS permite para o contexto fixado.
func TestINV5TraversalBarrier2(t *testing.T) {
	pool, fx := setupTraversal(t)
	ctx := context.Background()

	for _, stmt := range []string{
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + inv5Role + `') THEN
				CREATE ROLE ` + inv5Role + ` NOLOGIN NOBYPASSRLS;
			END IF;
		END $$;`,
		`GRANT USAGE ON SCHEMA public TO ` + inv5Role,
		`GRANT SELECT, INSERT, UPDATE ON membership, role_assignment, auth_session TO ` + inv5Role,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("grant papel: %v", err)
		}
	}

	// visibleAs conta, SEM predicado de tenant na query, se a linha `id` da
	// tabela é visível sob o contexto dado.
	visibleAs := func(t *testing.T, table string, id uuid.UUID, org *uuid.UUID) bool {
		t.Helper()
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
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+inv5Role); err != nil {
			t.Fatalf("set role: %v", err)
		}
		if org != nil {
			if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)",
				domain.RLSOrgSettingName, org.String()); err != nil {
				t.Fatalf("set org: %v", err)
			}
		}
		var n int
		if err := tx.QueryRow(ctx,
			"SELECT count(*) FROM "+table+" WHERE id = $1", id.String()).Scan(&n); err != nil {
			t.Fatalf("probe %s: %v", table, err)
		}
		return n == 1
	}

	probes := []struct {
		table    string
		inA, inB uuid.UUID
	}{
		{"membership", fx.memA, fx.memB},
		{"role_assignment", fx.assignA, uuid.Nil}, // só há assignment em A
		{"auth_session", fx.sessA, fx.sessB},
	}
	for _, p := range probes {
		// Contexto de A: vê a linha de A…
		if !visibleAs(t, p.table, p.inA, &fx.orgA) {
			t.Errorf("Barreira 2: %s de A deveria ser visível sob contexto de A", p.table)
		}
		// …e NÃO vê a de B (a query não tem predicado de aplicação nenhum).
		if p.inB != uuid.Nil && visibleAs(t, p.table, p.inB, &fx.orgA) {
			t.Errorf("Barreira 2: %s de B visível sob contexto de A — RLS falhou", p.table)
		}
		// Sem contexto nenhum: nada.
		if visibleAs(t, p.table, p.inA, nil) {
			t.Errorf("Barreira 2: %s visível SEM contexto de tenant", p.table)
		}
	}

	// ESCRITA contornando a aplicação: INSERT de linha de B sob contexto de A —
	// o WITH CHECK barra.
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
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+inv5Role); err != nil {
		t.Fatalf("set role: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)",
		domain.RLSOrgSettingName, fx.orgA.String()); err != nil {
		t.Fatalf("set org: %v", err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO role_assignment (id, organization_id, role_id, membership_id) VALUES (gen_random_uuid(), $1, $2, $3)",
		fx.orgB.String(), fx.roleA.String(), fx.memB.String()); err == nil {
		t.Errorf("Barreira 2: WITH CHECK deveria barrar escrita de linha de B sob contexto de A")
	}
}

// TestINV5RLSStaysEnabled trava o estado da Barreira 2: RLS LIGADA e FORÇADA
// nas tabelas novas. Se uma migration futura a desligar por engano, o build
// quebra aqui.
func TestINV5RLSStaysEnabled(t *testing.T) {
	pool, _ := setupTraversal(t)
	ctx := context.Background()
	for _, table := range []string{"membership", "role_assignment", "auth_session"} {
		var enabled, forced bool
		if err := pool.QueryRow(ctx,
			"SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE relname = $1", table).Scan(&enabled, &forced); err != nil {
			t.Fatalf("pg_class %s: %v", table, err)
		}
		if !enabled || !forced {
			t.Errorf("RLS de %s deveria estar ENABLE+FORCE, está enable=%v force=%v", table, enabled, forced)
		}
	}
}
