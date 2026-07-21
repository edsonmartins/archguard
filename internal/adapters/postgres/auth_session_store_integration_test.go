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

// sessionFixture: one identity holding memberships in two organizations — the
// shape of the "Múltiplos memberships no login" scenario — plus a second
// identity with a single membership in org A.
type sessionFixture struct {
	orgA, orgB   uuid.UUID
	identity     domain.Identity
	memA, memB   domain.Membership
	other        domain.Identity
	otherMemA    domain.Membership
	scopeIdn     domain.IdentityScope
	scopeOther   domain.IdentityScope
	tenantScopeA domain.TenantScope
	tenantScopeB domain.TenantScope
}

func makeSessionFixture(t *testing.T, pool *pgxpool.Pool, label string) sessionFixture {
	t.Helper()
	ctx := context.Background()
	var fx sessionFixture

	for name, dst := range map[string]*uuid.UUID{"a": &fx.orgA, "b": &fx.orgB} {
		if err := pool.QueryRow(ctx,
			"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id",
			"sess-org-"+name+"-"+label).Scan(dst); err != nil {
			t.Fatalf("insert organization %s: %v", name, err)
		}
	}

	newIdentity := func() domain.Identity {
		idn, err := domain.NewIdentity(domain.IdentityHuman)
		if err != nil {
			t.Fatalf("NewIdentity: %v", err)
		}
		if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
			t.Fatalf("cria identidade: %v", err)
		}
		return idn
	}
	insertMembership := func(identityID, orgID uuid.UUID) domain.Membership {
		m, err := domain.NewMembership(identityID, orgID)
		if err != nil {
			t.Fatalf("NewMembership: %v", err)
		}
		if _, err := pool.Exec(ctx,
			"INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1, $2, $3, $4)",
			m.ID.String(), m.IdentityID.String(), m.OrganizationID.String(), string(m.Status)); err != nil {
			t.Fatalf("insert membership: %v", err)
		}
		return m
	}

	fx.identity = newIdentity()
	fx.memA = insertMembership(fx.identity.ID, fx.orgA)
	fx.memB = insertMembership(fx.identity.ID, fx.orgB)
	fx.other = newIdentity()
	fx.otherMemA = insertMembership(fx.other.ID, fx.orgA)

	var err error
	if fx.scopeIdn, err = domain.NewIdentityScope(fx.identity.ID); err != nil {
		t.Fatalf("NewIdentityScope: %v", err)
	}
	if fx.scopeOther, err = domain.NewIdentityScope(fx.other.ID); err != nil {
		t.Fatalf("NewIdentityScope: %v", err)
	}
	if fx.tenantScopeA, err = domain.NewTenantScope(fx.orgA); err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	if fx.tenantScopeB, err = domain.NewTenantScope(fx.orgB); err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		for _, idn := range []uuid.UUID{fx.identity.ID, fx.other.ID} {
			_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE identity_id = $1", idn.String())
			_, _ = pool.Exec(bg, "DELETE FROM membership WHERE identity_id = $1", idn.String())
			_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", idn.String())
		}
		for _, org := range []uuid.UUID{fx.orgA, fx.orgB} {
			_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", org.String())
		}
	})
	return fx
}

// Um membership ativo: a sessão nasce ativa (auto-seleção) e persiste com o
// contexto de tenant resolvido.
func TestAuthSessionSingleMembershipAutoSelects(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "auto")

	sess, err := domain.NewAuthSession(fx.other.ID, domain.AAL2, []domain.Membership{fx.otherMemA})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	store := NewIdentitySessionStore(pool, fx.scopeOther)
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	mem, org, err := got.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant: %v", err)
	}
	if mem != fx.otherMemA.ID || org != fx.orgA {
		t.Fatalf("tenant ativo = (%s, %s), quero (%s, %s)", mem, org, fx.otherMemA.ID, fx.orgA)
	}
	if got.ProvenAAL != domain.AAL2 {
		t.Fatalf("ProvenAAL = %s, quero aal2", got.ProvenAAL)
	}
}

// Cenário "Múltiplos memberships no login": a sessão persiste pendente e sem
// tenant; a seleção explícita a resolve e é gravada.
func TestAuthSessionMultiMembershipRequiresSelection(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "multi")

	sess, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	store := NewIdentitySessionStore(pool, fx.scopeIdn)
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.SessionPendingSelection || got.MembershipID != nil || got.OrganizationID != nil {
		t.Fatalf("sessão deveria estar pendente e sem tenant: %+v", got)
	}
	if _, _, err := got.ActiveTenant(); !errors.Is(err, domain.ErrTenantSelectionRequired) {
		t.Fatalf("ActiveTenant pendente: err = %v, quero ErrTenantSelectionRequired", err)
	}

	if err := got.SelectTenant(fx.memB); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	if err := store.SaveSelection(ctx, got); err != nil {
		t.Fatalf("SaveSelection: %v", err)
	}

	// Reler: a seleção persistiu; repetir a gravação falha (não está mais pendente).
	reread, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get pós-seleção: %v", err)
	}
	mem, org, err := reread.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant pós-seleção: %v", err)
	}
	if mem != fx.memB.ID || org != fx.orgB {
		t.Fatalf("tenant ativo = (%s, %s), quero o selecionado (%s, %s)", mem, org, fx.memB.ID, fx.orgB)
	}
	if err := store.SaveSelection(ctx, got); !errors.Is(err, ErrSessionNotPending) {
		t.Fatalf("re-seleção: err = %v, quero ErrSessionNotPending", err)
	}
}

// O banco também recusa os atalhos: linha active sem tenant (CHECK) e tenant
// que não é membership desta identidade nesta organização (FK composta).
func TestAuthSessionDatabaseShapeGuards(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "shape")

	sess, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	store := NewIdentitySessionStore(pool, fx.scopeIdn)
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// CHECK auth_session_tenant_shape: active sem membership/org é rejeitado.
	if _, err := pool.Exec(ctx,
		"UPDATE auth_session SET status = 'active' WHERE id = $1", sess.ID.String()); err == nil {
		t.Fatalf("UPDATE para active sem tenant deveria violar o CHECK")
	}

	// FK composta: o "tenant ativo" precisa ser um membership DESTA identidade
	// NESTA organização — memB é da identidade, mas em B, não em A.
	if _, err := pool.Exec(ctx,
		`UPDATE auth_session SET status = 'active', membership_id = $2, organization_id = $3
		 WHERE id = $1`,
		sess.ID.String(), fx.memB.ID.String(), fx.orgA.String()); err == nil {
		t.Fatalf("membership de outra organização deveria violar a FK composta")
	}
	// E membership de OUTRA identidade também não.
	if _, err := pool.Exec(ctx,
		`UPDATE auth_session SET status = 'active', membership_id = $2, organization_id = $3
		 WHERE id = $1`,
		sess.ID.String(), fx.otherMemA.ID.String(), fx.orgA.String()); err == nil {
		t.Fatalf("membership de outra identidade deveria violar a FK composta")
	}
}

// Barreira 1 no eixo da identidade: o store de B não alcança nem escreve
// sessões de A.
func TestAuthSessionIdentityIsolation(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "isol")

	sess, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	own := NewIdentitySessionStore(pool, fx.scopeIdn)
	if err := own.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	foreign := NewIdentitySessionStore(pool, fx.scopeOther)
	if _, err := foreign.Get(ctx, sess.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Get alheio: err = %v, quero ErrSessionNotFound", err)
	}
	if err := foreign.Create(ctx, sess); !errors.Is(err, ErrCrossIdentityWrite) {
		t.Fatalf("Create alheio: err = %v, quero ErrCrossIdentityWrite", err)
	}
	if err := foreign.Revoke(ctx, sess.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Revoke alheio: err = %v, quero ErrSessionNotFound", err)
	}
}

// Lado do tenant (Barreira 1): a organização só enxerga as sessões cujo tenant
// ativo é ela — pendentes e sessões de outros tenants não aparecem.
func TestTenantSessionStoreListsOnlyOwnActiveSessions(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "tenant")

	// Sessão da identidade principal, ativa em B (seleção explícita).
	inB, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := inB.SelectTenant(fx.memB); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	storeIdn := NewIdentitySessionStore(pool, fx.scopeIdn)
	if err := storeIdn.Create(ctx, inB); err != nil {
		t.Fatalf("Create inB: %v", err)
	}
	// Sessão pendente da mesma identidade (sem tenant) — invisível para ambos.
	pending, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession pendente: %v", err)
	}
	if err := storeIdn.Create(ctx, pending); err != nil {
		t.Fatalf("Create pendente: %v", err)
	}
	// Sessão da outra identidade, ativa em A.
	inA, err := domain.NewAuthSession(fx.other.ID, domain.AAL1,
		[]domain.Membership{fx.otherMemA})
	if err != nil {
		t.Fatalf("NewAuthSession other: %v", err)
	}
	if err := NewIdentitySessionStore(pool, fx.scopeOther).Create(ctx, inA); err != nil {
		t.Fatalf("Create inA: %v", err)
	}

	list := func(scope domain.TenantScope) []domain.AuthSession {
		var out []domain.AuthSession
		repo := NewTenantRepository(pool, scope)
		if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
			var e error
			out, e = NewTenantSessionStore(ttx).ListActive(ctx)
			return e
		}); err != nil {
			t.Fatalf("ListActive: %v", err)
		}
		return out
	}

	gotA := list(fx.tenantScopeA)
	if len(gotA) != 1 || gotA[0].ID != inA.ID {
		t.Fatalf("org A: quero só a sessão de other em A, veio %d sessões", len(gotA))
	}
	gotB := list(fx.tenantScopeB)
	if len(gotB) != 1 || gotB[0].ID != inB.ID {
		t.Fatalf("org B: quero só a sessão ativa em B, veio %d sessões", len(gotB))
	}
}

// Revogação é terminal e idempotente; o contexto de tenant permanece na linha
// para a trilha.
func TestAuthSessionRevoke(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "revoke")

	sess, err := domain.NewAuthSession(fx.other.ID, domain.AAL1, []domain.Membership{fx.otherMemA})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	store := NewIdentitySessionStore(pool, fx.scopeOther)
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Revoke(ctx, sess.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	first, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if first.Status != domain.SessionRevoked || first.RevokedAt == nil {
		t.Fatalf("sessão deveria estar revogada com revoked_at: %+v", first)
	}
	if first.MembershipID == nil || *first.MembershipID != fx.otherMemA.ID {
		t.Fatalf("tenant da sessão revogada deve permanecer para a trilha")
	}

	// Idempotência: revogar de novo não erra nem move revoked_at.
	if err := store.Revoke(ctx, sess.ID); err != nil {
		t.Fatalf("Revoke idempotente: %v", err)
	}
	second, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}
	if !second.RevokedAt.Equal(*first.RevokedAt) {
		t.Fatalf("revoked_at não pode mudar na re-revogação: %v vs %v", second.RevokedAt, first.RevokedAt)
	}
	if _, _, err := second.ActiveTenant(); !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("ActiveTenant revogada: err = %v, quero ErrSessionRevoked", err)
	}
}
