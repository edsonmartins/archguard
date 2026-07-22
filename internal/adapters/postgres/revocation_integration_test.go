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
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedTwoTenantSessions persists one ACTIVE session per tenant (A and B) for
// fx.identity, returning them.
func seedTwoTenantSessions(t *testing.T, pool *pgxpool.Pool, fx sessionFixture) (inA, inB domain.AuthSession) {
	t.Helper()
	mk := func(m domain.Membership) domain.AuthSession {
		s, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
			[]domain.Membership{fx.memA, fx.memB})
		if err != nil {
			t.Fatalf("NewAuthSession: %v", err)
		}
		if err := s.SelectTenant(m); err != nil {
			t.Fatalf("SelectTenant: %v", err)
		}
		if err := createSession(pool, fx.scopeIdn, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		return s
	}
	return mk(fx.memA), mk(fx.memB)
}

func membershipStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		"SELECT status FROM membership WHERE id = $1", id.String()).Scan(&status); err != nil {
		t.Fatalf("status de membership: %v", err)
	}
	return status
}

func sessionStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		"SELECT status FROM auth_session WHERE id = $1", id.String()).Scan(&status); err != nil {
		t.Fatalf("status de sessão: %v", err)
	}
	return status
}

// Cenário "Revogação de membership": revogar o membership em A encerra as
// sessões da identidade NO TENANT A — e os memberships e sessões nas outras
// organizações permanecem ativos.
func TestRevokeMembershipCascadesOnlyItsTenant(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "rvkmem")
	inA, inB := seedTwoTenantSessions(t, pool, fx)

	revoker := NewMembershipRevoker(NewTenantRepository(pool, fx.tenantScopeA), nil)
	m, sessions, err := revoker.RevokeMembership(ctx, fx.memA.ID)
	if err != nil {
		t.Fatalf("RevokeMembership: %v", err)
	}
	if m.Status != domain.MembershipRevoked || sessions != 1 {
		t.Fatalf("quero membership revogado + 1 sessão encerrada, veio %s/%d", m.Status, sessions)
	}

	// O tenant A: membership revogado (com revoked_at), sessão encerrada.
	if got := membershipStatus(t, pool, fx.memA.ID); got != "revoked" {
		t.Fatalf("membership A = %s, quero revoked", got)
	}
	if got := sessionStatus(t, pool, inA.ID); got != "revoked" {
		t.Fatalf("sessão em A = %s, quero revoked", got)
	}
	// O AND da spec: B permanece intacto.
	if got := membershipStatus(t, pool, fx.memB.ID); got != "active" {
		t.Fatalf("membership B = %s, deveria permanecer active", got)
	}
	if got := sessionStatus(t, pool, inB.ID); got != "active" {
		t.Fatalf("sessão em B = %s, deveria permanecer active", got)
	}

	// Idempotência ponta a ponta: revogar de novo não erra e não encerra nada novo.
	if _, sessions, err = revoker.RevokeMembership(ctx, fx.memA.ID); err != nil || sessions != 0 {
		t.Fatalf("re-revogação: err=%v sessões=%d, quero nil/0", err, sessions)
	}

	// Membership de outro tenant é inalcançável pelo revoker de A.
	if _, _, err := revoker.RevokeMembership(ctx, fx.memB.ID); !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("revogação cross-tenant: err = %v, quero ErrMembershipNotFound", err)
	}
}

// Cenário "Suspensão da identidade": todos os memberships ativos são suspensos
// (recuperáveis — decisão do arquiteto) e TODAS as sessões (inclusive a
// pendente) são encerradas, em uma transação.
func TestSuspendIdentityCascades(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "suspend")
	inA, inB := seedTwoTenantSessions(t, pool, fx)

	// Uma sessão pendente também cai na cascata.
	pending, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := createSession(pool, fx.scopeIdn, pending); err != nil {
		t.Fatalf("Create pendente: %v", err)
	}

	report, err := NewIdentityLifecycle(NewIdentityRepository(pool, fx.scopeIdn), nil).Suspend(ctx)
	if err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if report.Identity.Status != domain.IdentitySuspended {
		t.Fatalf("identidade = %s, quero suspended", report.Identity.Status)
	}
	if report.MembershipsMoved != 2 || report.SessionsRevoked != 3 {
		t.Fatalf("cascata: memberships=%d sessões=%d, quero 2/3", report.MembershipsMoved, report.SessionsRevoked)
	}
	for _, m := range []uuid.UUID{fx.memA.ID, fx.memB.ID} {
		if got := membershipStatus(t, pool, m); got != "suspended" {
			t.Fatalf("membership %s = %s, quero suspended (recuperável)", m, got)
		}
	}
	for _, s := range []uuid.UUID{inA.ID, inB.ID, pending.ID} {
		if got := sessionStatus(t, pool, s); got != "revoked" {
			t.Fatalf("sessão %s = %s, quero revoked", s, got)
		}
	}

	// A identidade do outro usuário não é tocada (Barreira 1 do eixo identidade).
	if got := membershipStatus(t, pool, fx.otherMemA.ID); got != "active" {
		t.Fatalf("membership de outra identidade = %s, deveria permanecer active", got)
	}
}

// R4 pleno: deprovisionar revoga TODOS os memberships (terminal) e encerra
// todas as sessões; a identidade fica no terminal R5.
func TestDeprovisionIdentityCascades(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "deprov")
	inA, inB := seedTwoTenantSessions(t, pool, fx)

	lifecycle := NewIdentityLifecycle(NewIdentityRepository(pool, fx.scopeIdn), nil)
	report, err := lifecycle.Deprovision(ctx)
	if err != nil {
		t.Fatalf("Deprovision: %v", err)
	}
	if report.Identity.Status != domain.IdentityDeprovisioned {
		t.Fatalf("identidade = %s, quero deprovisioned", report.Identity.Status)
	}
	if report.MembershipsMoved != 2 || report.SessionsRevoked != 2 {
		t.Fatalf("cascata: memberships=%d sessões=%d, quero 2/2", report.MembershipsMoved, report.SessionsRevoked)
	}
	for _, m := range []uuid.UUID{fx.memA.ID, fx.memB.ID} {
		if got := membershipStatus(t, pool, m); got != "revoked" {
			t.Fatalf("membership %s = %s, quero revoked (terminal)", m, got)
		}
	}
	for _, s := range []uuid.UUID{inA.ID, inB.ID} {
		if got := sessionStatus(t, pool, s); got != "revoked" {
			t.Fatalf("sessão %s = %s, quero revoked", s, got)
		}
	}

	// Terminal R5: suspender depois de deprovisionar é recusado.
	if _, err := lifecycle.Suspend(ctx); !errors.Is(err, domain.ErrIdentityDeprovisioned) {
		t.Fatalf("suspensão pós-deprovisionamento: err = %v, quero ErrIdentityDeprovisioned", err)
	}
	// Idempotência do deprovisionamento: nada novo a mover.
	again, err := lifecycle.Deprovision(ctx)
	if err != nil || again.MembershipsMoved != 0 || again.SessionsRevoked != 0 {
		t.Fatalf("re-deprovisionamento: err=%v memberships=%d sessões=%d, quero nil/0/0",
			err, again.MembershipsMoved, again.SessionsRevoked)
	}
}

// Barreira 2 da 0014, como papel não-superusuário: o eixo de identidade só
// alcança as PRÓPRIAS linhas de membership; o eixo de tenant segue funcionando;
// WITH CHECK barra escrita em membership alheio.
func TestMembershipRLSIdentityAxis(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	grantRLSRole(t, pool)
	fx := makeSessionFixture(t, pool, "rlsidn")

	countVisible := func(t *testing.T, identityCtx *uuid.UUID, orgCtx *uuid.UUID) (own, foreign bool) {
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
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+rlsTestRole); err != nil {
			t.Fatalf("set role: %v", err)
		}
		if identityCtx != nil {
			if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)",
				domain.RLSIdentitySettingName, identityCtx.String()); err != nil {
				t.Fatalf("set identity: %v", err)
			}
		}
		if orgCtx != nil {
			if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)",
				domain.RLSOrgSettingName, orgCtx.String()); err != nil {
				t.Fatalf("set org: %v", err)
			}
		}
		for _, probe := range []struct {
			id  uuid.UUID
			dst *bool
		}{{fx.memB.ID, &own}, {fx.otherMemA.ID, &foreign}} {
			var n int
			if err := tx.QueryRow(ctx,
				"SELECT count(*) FROM membership WHERE id = $1", probe.id.String()).Scan(&n); err != nil {
				t.Fatalf("count: %v", err)
			}
			*probe.dst = n == 1
		}
		return own, foreign
	}

	// Eixo identidade: vê o próprio membership em B, não o de outra identidade.
	if own, foreign := countVisible(t, &fx.identity.ID, nil); !own || foreign {
		t.Fatalf("eixo identidade: own=%v foreign=%v, quero true/false", own, foreign)
	}
	// Eixo tenant (org A): vê o membership alheio DE A, não o próprio em B.
	if own, foreign := countVisible(t, nil, &fx.orgA); own || !foreign {
		t.Fatalf("eixo tenant A: own-em-B=%v foreign-em-A=%v, quero false/true", own, foreign)
	}
	// Sem contexto: nada.
	if own, foreign := countVisible(t, nil, nil); own || foreign {
		t.Fatalf("sem contexto: own=%v foreign=%v, quero false/false", own, foreign)
	}

	// Escrita pelo eixo identidade: suspender os PRÓPRIOS memberships passa;
	// linha de outra identidade fica invisível ao UPDATE (0 linhas).
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
	if _, err := tx.Exec(ctx, "SELECT set_config($1,$2,true)",
		domain.RLSIdentitySettingName, fx.identity.ID.String()); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	tag, err := tx.Exec(ctx,
		"UPDATE membership SET status = 'suspended', updated_at = now() WHERE identity_id = $1 AND status = 'active'",
		fx.identity.ID.String())
	if err != nil {
		t.Fatalf("cascata como papel app: %v", err)
	}
	if tag.RowsAffected() != 2 {
		t.Fatalf("cascata deveria alcançar os 2 memberships próprios, alcançou %d", tag.RowsAffected())
	}
	tag, err = tx.Exec(ctx,
		"UPDATE membership SET status = 'suspended', updated_at = now() WHERE id = $1",
		fx.otherMemA.ID.String())
	if err != nil {
		t.Fatalf("update alheio: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Fatalf("membership alheio deveria ser invisível ao UPDATE, alcançou %d", tag.RowsAffected())
	}
}
