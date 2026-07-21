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

// --- helpers: each store operation in one identity-pinned transaction ---

func inIdentityTx(pool *pgxpool.Pool, scope domain.IdentityScope, fn func(*IdentitySessionStore) error) error {
	return NewIdentityRepository(pool, scope).WithIdentityTx(context.Background(),
		func(itx *IdentityTx) error { return fn(NewIdentitySessionStore(itx)) })
}

func createSession(pool *pgxpool.Pool, scope domain.IdentityScope, as domain.AuthSession) error {
	return inIdentityTx(pool, scope, func(s *IdentitySessionStore) error {
		return s.Create(context.Background(), as)
	})
}

func getSession(pool *pgxpool.Pool, scope domain.IdentityScope, id uuid.UUID) (domain.AuthSession, error) {
	var out domain.AuthSession
	err := inIdentityTx(pool, scope, func(s *IdentitySessionStore) error {
		var e error
		out, e = s.Get(context.Background(), id)
		return e
	})
	return out, err
}

// --- test doubles for the tenant-switch ports ---

type staticAALPolicy struct {
	aal domain.AAL
	err error
}

func (p staticAALPolicy) RequiredAAL(_ context.Context, _ uuid.UUID) (domain.AAL, error) {
	return p.aal, p.err
}

// outboxCount returns how many tenant-switch events the outbox holds for a
// session — the durable, in-transaction audit record the switch writes.
func outboxCount(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM session_event_outbox WHERE event_type = 'tenant_switch' AND session_id = $1",
		sessionID.String()).Scan(&n); err != nil {
		t.Fatalf("contagem do outbox: %v", err)
	}
	return n
}

// Um membership ativo: a sessão nasce ativa (auto-seleção) e persiste com o
// contexto de tenant resolvido.
func TestAuthSessionSingleMembershipAutoSelects(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "auto")

	sess, err := domain.NewAuthSession(fx.other.ID, domain.AAL2, []domain.Membership{fx.otherMemA})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := createSession(pool, fx.scopeOther, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := getSession(pool, fx.scopeOther, sess.ID)
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
	if got.ProvenAAL != domain.AAL2 || got.TokenGeneration != 1 {
		t.Fatalf("aal/geração = %s/%d, quero aal2/1", got.ProvenAAL, got.TokenGeneration)
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
	if err := createSession(pool, fx.scopeIdn, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := getSession(pool, fx.scopeIdn, sess.ID)
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
	if err := inIdentityTx(pool, fx.scopeIdn, func(s *IdentitySessionStore) error {
		return s.SaveSelection(ctx, got)
	}); err != nil {
		t.Fatalf("SaveSelection: %v", err)
	}

	// Reler: a seleção persistiu; repetir a gravação falha (não está mais pendente).
	reread, err := getSession(pool, fx.scopeIdn, sess.ID)
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
	if err := inIdentityTx(pool, fx.scopeIdn, func(s *IdentitySessionStore) error {
		return s.SaveSelection(ctx, got)
	}); !errors.Is(err, ErrSessionNotPending) {
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
	if err := createSession(pool, fx.scopeIdn, sess); err != nil {
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
	if err := createSession(pool, fx.scopeIdn, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := getSession(pool, fx.scopeOther, sess.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Get alheio: err = %v, quero ErrSessionNotFound", err)
	}
	if err := createSession(pool, fx.scopeOther, sess); !errors.Is(err, ErrCrossIdentityWrite) {
		t.Fatalf("Create alheio: err = %v, quero ErrCrossIdentityWrite", err)
	}
	if err := inIdentityTx(pool, fx.scopeOther, func(s *IdentitySessionStore) error {
		return s.Revoke(ctx, sess.ID)
	}); !errors.Is(err, ErrSessionNotFound) {
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
	if err := createSession(pool, fx.scopeIdn, inB); err != nil {
		t.Fatalf("Create inB: %v", err)
	}
	// Sessão pendente da mesma identidade (sem tenant) — invisível para ambos.
	pending, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession pendente: %v", err)
	}
	if err := createSession(pool, fx.scopeIdn, pending); err != nil {
		t.Fatalf("Create pendente: %v", err)
	}
	// Sessão da outra identidade, ativa em A.
	inA, err := domain.NewAuthSession(fx.other.ID, domain.AAL1,
		[]domain.Membership{fx.otherMemA})
	if err != nil {
		t.Fatalf("NewAuthSession other: %v", err)
	}
	if err := createSession(pool, fx.scopeOther, inA); err != nil {
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
	if err := createSession(pool, fx.scopeOther, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	revoke := func() error {
		return inIdentityTx(pool, fx.scopeOther, func(s *IdentitySessionStore) error {
			return s.Revoke(ctx, sess.ID)
		})
	}
	if err := revoke(); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	first, err := getSession(pool, fx.scopeOther, sess.ID)
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
	if err := revoke(); err != nil {
		t.Fatalf("Revoke idempotente: %v", err)
	}
	second, err := getSession(pool, fx.scopeOther, sess.ID)
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

// --- T-012: troca de tenant ---

// activeSessionInA persists a session for fx.identity with tenant A selected.
func activeSessionInA(t *testing.T, pool *pgxpool.Pool, fx sessionFixture) domain.AuthSession {
	t.Helper()
	sess, err := domain.NewAuthSession(fx.identity.ID, domain.AAL1,
		[]domain.Membership{fx.memA, fx.memB})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := sess.SelectTenant(fx.memA); err != nil {
		t.Fatalf("SelectTenant: %v", err)
	}
	if err := createSession(pool, fx.scopeIdn, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return sess
}

// Cenário "Troca de tenant": novo contexto com o org do destino, geração de
// token incrementada (o token anterior nunca coincide com a geração corrente) e
// evento de auditoria registrado com origem e destino.
func TestTenantSwitchReissuesAndAudits(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "sw")
	sess := activeSessionInA(t, pool, fx)

	before, err := sess.TokenContext(fx.identity)
	if err != nil {
		t.Fatalf("TokenContext antes: %v", err)
	}

	sw := NewTenantSwitcher(NewIdentityRepository(pool, fx.scopeIdn),
		staticAALPolicy{aal: domain.AAL1})
	got, err := sw.Switch(ctx, sess.ID, fx.memB)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	// O que voltou e o que está no banco: destino B, geração 2.
	persisted, err := getSession(pool, fx.scopeIdn, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	for name, s := range map[string]domain.AuthSession{"retorno": got, "banco": persisted} {
		mem, org, err := s.ActiveTenant()
		if err != nil {
			t.Fatalf("%s: ActiveTenant: %v", name, err)
		}
		if mem != fx.memB.ID || org != fx.orgB {
			t.Fatalf("%s: tenant = (%s, %s), quero o destino B", name, mem, org)
		}
		if s.TokenGeneration != 2 {
			t.Fatalf("%s: geração = %d, quero 2", name, s.TokenGeneration)
		}
	}

	// O novo token carrega o org do destino e geração nova — o anterior (geração
	// 1, org A) nunca valida contra a sessão atual.
	after, err := got.TokenContext(fx.identity)
	if err != nil {
		t.Fatalf("TokenContext depois: %v", err)
	}
	if after.OrganizationID != fx.orgB || after.TokenGeneration <= before.TokenGeneration {
		t.Fatalf("claims novos errados: %+v (antes %+v)", after, before)
	}

	// Evento de auditoria no outbox transacional: exatamente um, de A para B,
	// com a geração nova (escrito na MESMA transação da troca).
	if n := outboxCount(t, pool, sess.ID); n != 1 {
		t.Fatalf("quero 1 evento de troca no outbox, veio %d", n)
	}
	var fromOrg, toOrg string
	var gen int
	if err := pool.QueryRow(ctx,
		`SELECT from_organization_id::text, to_organization_id::text, token_generation
		 FROM session_event_outbox WHERE session_id = $1`, sess.ID.String()).Scan(&fromOrg, &toOrg, &gen); err != nil {
		t.Fatalf("leitura do outbox: %v", err)
	}
	if fromOrg != fx.orgA.String() || toOrg != fx.orgB.String() || gen != 2 {
		t.Fatalf("evento errado no outbox: from=%s to=%s gen=%d", fromOrg, toOrg, gen)
	}

	// Mesmo destino de novo: não é troca — recusa sem novo evento.
	if _, err := sw.Switch(ctx, sess.ID, fx.memB); !errors.Is(err, domain.ErrSameTenant) {
		t.Fatalf("mesmo tenant: err = %v, quero ErrSameTenant", err)
	}
	if n := outboxCount(t, pool, sess.ID); n != 1 {
		t.Fatalf("recusa não pode gerar evento: %d", n)
	}
}

// Cenário "Política mais restritiva no destino": destino exige AAL3, sessão
// comprovou AAL1 ⇒ troca negada, nada muda, nada é auditado como troca.
func TestTenantSwitchStepUpDenied(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "stepup")
	sess := activeSessionInA(t, pool, fx)

	sw := NewTenantSwitcher(NewIdentityRepository(pool, fx.scopeIdn),
		staticAALPolicy{aal: domain.AAL3})
	if _, err := sw.Switch(ctx, sess.ID, fx.memB); !errors.Is(err, domain.ErrStepUpRequired) {
		t.Fatalf("Switch: err = %v, quero ErrStepUpRequired", err)
	}

	persisted, err := getSession(pool, fx.scopeIdn, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	mem, _, err := persisted.ActiveTenant()
	if err != nil || mem != fx.memA.ID || persisted.TokenGeneration != 1 {
		t.Fatalf("troca negada não pode alterar a sessão: mem=%s gen=%d err=%v",
			mem, persisted.TokenGeneration, err)
	}
	// I-5.4 estrutural: troca negada não escreve no outbox (mesma transação).
	if n := outboxCount(t, pool, sess.ID); n != 0 {
		t.Fatalf("troca negada não pode ser auditada como troca: %d eventos no outbox", n)
	}
}

// INV-6: política do destino indisponível ⇒ negação, nunca fail-open.
func TestTenantSwitchPolicyErrorDenies(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "pol")
	sess := activeSessionInA(t, pool, fx)

	sw := NewTenantSwitcher(NewIdentityRepository(pool, fx.scopeIdn),
		staticAALPolicy{err: errors.New("pdp fora do ar")})
	if _, err := sw.Switch(ctx, sess.ID, fx.memB); !errors.Is(err, domain.ErrDestinationPolicyUnavailable) {
		t.Fatalf("Switch: err = %v, quero ErrDestinationPolicyUnavailable", err)
	}

	persisted, err := getSession(pool, fx.scopeIdn, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mem, _, _ := persisted.ActiveTenant(); mem != fx.memA.ID || persisted.TokenGeneration != 1 {
		t.Fatalf("negação por política não pode alterar a sessão")
	}
}

// TOCTOU: o destino é validado em memória (snapshot do chamador), mas o
// SaveSwitch re-confere no banco, na mesma transação, que a membership de
// destino ainda está ativa. Se ela foi revogada no intervalo, a troca é negada
// (ErrSwitchConflict) e a sessão não se vincula a uma membership morta.
func TestTenantSwitchToRevokedMembershipDenied(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "swrevk")
	sess := activeSessionInA(t, pool, fx)

	// memB é revogada no banco DEPOIS que o chamador tomou o snapshot ativo.
	if _, err := pool.Exec(ctx,
		"UPDATE membership SET status = 'revoked', revoked_at = now() WHERE id = $1", fx.memB.ID.String()); err != nil {
		t.Fatalf("revoga memB: %v", err)
	}

	// fx.memB (snapshot) ainda está 'active' em memória — a checagem de domínio
	// passa, mas o SaveSwitch re-checa o banco e recusa.
	sw := NewTenantSwitcher(NewIdentityRepository(pool, fx.scopeIdn),
		staticAALPolicy{aal: domain.AAL1})
	if _, err := sw.Switch(ctx, sess.ID, fx.memB); !errors.Is(err, ErrSwitchConflict) {
		t.Fatalf("troca para membership revogada: err = %v, quero ErrSwitchConflict", err)
	}

	// A sessão permanece em A, geração 1, e nada foi ao outbox.
	persisted, err := getSession(pool, fx.scopeIdn, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mem, _, _ := persisted.ActiveTenant(); mem != fx.memA.ID || persisted.TokenGeneration != 1 {
		t.Fatalf("troca negada não pode alterar a sessão")
	}
	if n := outboxCount(t, pool, sess.ID); n != 0 {
		t.Fatalf("troca negada não pode auditar: %d eventos", n)
	}
}

// I-5.4 estrutural: o evento vai ao outbox na MESMA transação da troca, logo os
// dois commitam ou dão rollback juntos. Se algo após o SaveSwitch falha, a troca
// E o evento são desfeitos — nunca uma troca sem registro, nem um registro sem
// troca. Provado forçando um erro dentro da transação após ambas as escritas.
func TestTenantSwitchOutboxIsAtomic(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "atomic")
	sess := activeSessionInA(t, pool, fx)

	moved := sess
	ev, err := moved.SwitchTenant(fx.memB, domain.AAL1)
	if err != nil {
		t.Fatalf("SwitchTenant: %v", err)
	}
	boom := errors.New("falha após as escritas")
	err = NewIdentityRepository(pool, fx.scopeIdn).WithIdentityTx(ctx, func(itx *IdentityTx) error {
		if err := NewIdentitySessionStore(itx).SaveSwitch(ctx, moved, fx.memA.ID, 1); err != nil {
			return err
		}
		if err := NewSessionOutbox(itx.Tx()).EnqueueTenantSwitch(ctx, ev); err != nil {
			return err
		}
		return boom // força o rollback com ambas as escritas já emitidas
	})
	if !errors.Is(err, boom) {
		t.Fatalf("esperava o erro forçado, veio %v", err)
	}

	// A sessão não moveu e o outbox está vazio — as duas escritas foram desfeitas.
	persisted, err := getSession(pool, fx.scopeIdn, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	mem, org, err := persisted.ActiveTenant()
	if err != nil {
		t.Fatalf("ActiveTenant: %v", err)
	}
	if mem != fx.memA.ID || org != fx.orgA || persisted.TokenGeneration != 1 {
		t.Fatalf("troca desfeita deveria manter a sessão em A: mem=%s org=%s gen=%d",
			mem, org, persisted.TokenGeneration)
	}
	if n := outboxCount(t, pool, sess.ID); n != 0 {
		t.Fatalf("rollback deveria descartar o evento do outbox, restaram %d", n)
	}
}

// A gravação da troca é otimista: origem/geração esperadas que não batem com o
// banco (sessão mudou concorrentemente) não atualizam nada.
func TestSaveSwitchConflict(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "conflict")
	sess := activeSessionInA(t, pool, fx)

	moved := sess
	if _, err := moved.SwitchTenant(fx.memB, domain.AAL1); err != nil {
		t.Fatalf("SwitchTenant: %v", err)
	}

	// Geração esperada errada (como se outra troca tivesse ocorrido no meio).
	if err := inIdentityTx(pool, fx.scopeIdn, func(s *IdentitySessionStore) error {
		return s.SaveSwitch(ctx, moved, fx.memA.ID, 99)
	}); !errors.Is(err, ErrSwitchConflict) {
		t.Fatalf("geração divergente: err = %v, quero ErrSwitchConflict", err)
	}
	// Origem esperada errada.
	if err := inIdentityTx(pool, fx.scopeIdn, func(s *IdentitySessionStore) error {
		return s.SaveSwitch(ctx, moved, fx.memB.ID, 1)
	}); !errors.Is(err, ErrSwitchConflict) {
		t.Fatalf("origem divergente: err = %v, quero ErrSwitchConflict", err)
	}
	// Esperados corretos aplicam.
	if err := inIdentityTx(pool, fx.scopeIdn, func(s *IdentitySessionStore) error {
		return s.SaveSwitch(ctx, moved, fx.memA.ID, 1)
	}); err != nil {
		t.Fatalf("SaveSwitch: %v", err)
	}
}

// --- Barreira 2 (RLS da auth_session, migration 0013), como papel não-superusuário ---

func TestAuthSessionRLSIsolation(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	grantRLSRole(t, pool)
	fx := makeSessionFixture(t, pool, "rls")

	// Semeia como SUPERUSUÁRIO (ignora RLS): uma sessão PENDENTE da identidade
	// principal (sem organização) e uma ATIVA da outra identidade em A.
	pendingID := uuid.New()
	if _, err := pool.Exec(ctx,
		"INSERT INTO auth_session (id, identity_id, status, proven_aal) VALUES ($1, $2, 'pending_selection', 'aal1')",
		pendingID.String(), fx.identity.ID.String()); err != nil {
		t.Fatalf("seed pendente: %v", err)
	}
	activeID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal)
		 VALUES ($1, $2, $3, $4, 'active', 'aal1')`,
		activeID.String(), fx.other.ID.String(), fx.otherMemA.ID.String(), fx.orgA.String()); err != nil {
		t.Fatalf("seed ativa: %v", err)
	}

	// visible executa como o papel NOBYPASSRLS com os contextos dados e devolve
	// quais das duas sessões a RLS deixa ver.
	visible := func(t *testing.T, identityCtx, orgCtx *uuid.UUID, globalRead bool) (pending, active bool) {
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
		if globalRead {
			if _, err := tx.Exec(ctx, "SELECT set_config($1,'on',true)",
				domain.RLSGlobalReadSettingName); err != nil {
				t.Fatalf("set global: %v", err)
			}
		}
		for _, probe := range []struct {
			id  uuid.UUID
			dst *bool
		}{{pendingID, &pending}, {activeID, &active}} {
			var n int
			if err := tx.QueryRow(ctx,
				"SELECT count(*) FROM auth_session WHERE id = $1", probe.id.String()).Scan(&n); err != nil {
				t.Fatalf("count: %v", err)
			}
			*probe.dst = n == 1
		}
		return pending, active
	}

	// Contexto da identidade principal: vê a própria pendente, não a alheia.
	if p, a := visible(t, &fx.identity.ID, nil, false); !p || a {
		t.Fatalf("contexto identidade: pendente=%v ativa-alheia=%v, quero true/false", p, a)
	}
	// Contexto da outra identidade: vê a própria ativa, não a pendente alheia.
	if p, a := visible(t, &fx.other.ID, nil, false); p || !a {
		t.Fatalf("contexto other: pendente-alheia=%v ativa=%v, quero false/true", p, a)
	}
	// Contexto de TENANT (org A, sem identidade): vê a ativa em A; a pendente
	// não tem organização e só é alcançável pelo eixo da identidade.
	if p, a := visible(t, nil, &fx.orgA, false); p || !a {
		t.Fatalf("contexto org A: pendente=%v ativa=%v, quero false/true", p, a)
	}
	// Sem contexto nenhum: nada.
	if p, a := visible(t, nil, nil, false); p || a {
		t.Fatalf("sem contexto: pendente=%v ativa=%v, quero false/false", p, a)
	}
	// Leitura global (autorizada+auditada, T-009): tudo.
	if p, a := visible(t, nil, nil, true); !p || !a {
		t.Fatalf("leitura global: pendente=%v ativa=%v, quero true/true", p, a)
	}

	// WITH CHECK: no contexto da identidade principal, inserir sessão de OUTRA
	// identidade é barrado pela política (mesmo com o predicado de aplicação
	// contornado — Barreira 2 isolada).
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
	if _, err := tx.Exec(ctx,
		"INSERT INTO auth_session (id, identity_id, status, proven_aal) VALUES ($1, $2, 'pending_selection', 'aal1')",
		uuid.New().String(), fx.other.ID.String()); err == nil {
		t.Fatalf("WITH CHECK deveria barrar escrita de sessão de outra identidade")
	}
}
