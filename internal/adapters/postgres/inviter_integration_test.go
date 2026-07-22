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

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// inviteFixture: identity X (with e-mail) already member of org A; org B is the
// tenant issuing the invitation — the exact shape of the spec scenario
// "Vinculação a nova organização".
type inviteFixture struct {
	orgA, orgB uuid.UUID
	email      string
	identity   domain.Identity
	memA       domain.Membership
	inviter    uuid.UUID // identity that issues the invite (an admin of B)
	scopeB     domain.TenantScope
	custodian  domain.KeyCustodian
}

func makeInviteFixture(t *testing.T, pool *pgxpool.Pool, label string) inviteFixture {
	t.Helper()
	ctx := context.Background()
	var fx inviteFixture
	fx.email = "invite-" + label + "@example.com"

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cust, err := keycustodian.NewProvisional(key)
	if err != nil {
		t.Fatalf("custodian: %v", err)
	}
	fx.custodian = cust

	for name, dst := range map[string]*uuid.UUID{"a": &fx.orgA, "b": &fx.orgB} {
		if err := pool.QueryRow(ctx,
			"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id",
			"inv-org-"+name+"-"+label).Scan(dst); err != nil {
			t.Fatalf("insert organization %s: %v", name, err)
		}
	}

	fx.identity = newIdentityWithEmail(t, cust, fx.email)
	if err := NewIdentityStore(pool).Create(ctx, fx.identity); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	if fx.memA, err = domain.NewMembership(fx.identity.ID, fx.orgA); err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1, $2, $3, $4)",
		fx.memA.ID.String(), fx.identity.ID.String(), fx.orgA.String(), string(fx.memA.Status)); err != nil {
		t.Fatalf("insert membership A: %v", err)
	}

	// The inviting admin of B: a second identity, member of B.
	admin, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity admin: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, admin); err != nil {
		t.Fatalf("cria admin: %v", err)
	}
	fx.inviter = admin.ID
	if _, err := pool.Exec(ctx,
		"INSERT INTO membership (id, identity_id, organization_id, status) VALUES (gen_random_uuid(), $1, $2, 'active')",
		admin.ID.String(), fx.orgB.String()); err != nil {
		t.Fatalf("insert membership admin: %v", err)
	}

	if fx.scopeB, err = domain.NewTenantScope(fx.orgB); err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		for _, idn := range []uuid.UUID{fx.identity.ID, fx.inviter} {
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

func countIdentities(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM identity").Scan(&n); err != nil {
		t.Fatalf("count identity: %v", err)
	}
	return n
}

// Cenário "Vinculação a nova organização": convidar e-mail já associado cria
// um NOVO MEMBERSHIP para a identidade existente — e NÃO uma nova identidade.
func TestInviteLinksExistingIdentityWithoutCreatingOne(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "link")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	before := countIdentities(t, pool)
	m, err := inv.InviteByEmail(ctx, fx.email, fx.inviter)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if after := countIdentities(t, pool); after != before {
		t.Fatalf("convite criou identidade: %d → %d (NÃO pode)", before, after)
	}

	// O membership é da identidade EXISTENTE, no tenant convidante, invited,
	// com o convidante registrado.
	if m.IdentityID != fx.identity.ID {
		t.Fatalf("membership de %s, quero a identidade existente %s", m.IdentityID, fx.identity.ID)
	}
	if m.OrganizationID != fx.orgB || m.Status != domain.MembershipInvited {
		t.Fatalf("membership errado: org=%s status=%s", m.OrganizationID, m.Status)
	}
	if m.InvitedBy == nil || *m.InvitedBy != fx.inviter {
		t.Fatalf("invited_by não registrado: %v", m.InvitedBy)
	}

	// Persistiu no banco no tenant B, e o membership em A segue intacto —
	// a identidade agora tem DOIS vínculos e UM conjunto de credenciais.
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM membership WHERE identity_id = $1", fx.identity.ID.String()).Scan(&n); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if n != 2 {
		t.Fatalf("quero 2 memberships (A ativo + B invited), veio %d", n)
	}

	// O convite é case-insensitive como o login (hash sobre e-mail normalizado).
	fx2 := makeInviteFixture(t, pool, "case")
	inv2 := NewInviter(NewTenantRepository(pool, fx2.scopeB), fx2.custodian, nil)
	if _, err := inv2.InviteByEmail(ctx, "  INVITE-case@EXAMPLE.com ", fx2.inviter); err != nil {
		t.Fatalf("convite com e-mail em caixa alta deveria achar a identidade: %v", err)
	}
}

// Decisão do arquiteto: e-mail sem identidade correspondente NÃO cria nada —
// resultado distinto, criação de identidade é dos pacotes 008/009.
func TestInviteUnknownEmailRefusedWithoutSideEffects(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "unknown")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	before := countIdentities(t, pool)
	_, err := inv.InviteByEmail(ctx, "ninguem-"+uuid.NewString()+"@example.com", fx.inviter)
	if !errors.Is(err, ErrUnknownInviteEmail) {
		t.Fatalf("err = %v, quero ErrUnknownInviteEmail", err)
	}
	if after := countIdentities(t, pool); after != before {
		t.Fatalf("recusa não pode criar identidade: %d → %d", before, after)
	}
}

// R3: convidar quem já é membro (ou já foi convidado) é recusado com erro
// classificado; par com membership revogado é recusado como readmissão (R3
// estrita — emenda do RFC é pré-requisito para readmitir).
func TestInviteR3Collisions(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "r3")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	if _, err := inv.InviteByEmail(ctx, fx.email, fx.inviter); err != nil {
		t.Fatalf("primeiro convite: %v", err)
	}
	// Segundo convite do mesmo par: já convidado.
	if _, err := inv.InviteByEmail(ctx, fx.email, fx.inviter); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("re-convite: err = %v, quero ErrAlreadyMember", err)
	}

	// Par com membership REVOGADO: readmissão barrada pela R3 estrita.
	if _, err := pool.Exec(ctx,
		"UPDATE membership SET status = 'revoked', revoked_at = now() WHERE identity_id = $1 AND organization_id = $2",
		fx.identity.ID.String(), fx.orgB.String()); err != nil {
		t.Fatalf("revoga membership: %v", err)
	}
	if _, err := inv.InviteByEmail(ctx, fx.email, fx.inviter); !errors.Is(err, ErrPreviouslyRevoked) {
		t.Fatalf("convite pós-revogação: err = %v, quero ErrPreviouslyRevoked", err)
	}
}

// Identidade suspensa/deprovisionada não ganha tenant novo (fail-closed).
func TestInviteNonActiveIdentityRefused(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "inactive")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	for _, status := range []string{"suspended", "deprovisioned"} {
		if _, err := pool.Exec(ctx,
			"UPDATE identity SET status = $2 WHERE id = $1",
			fx.identity.ID.String(), status); err != nil {
			t.Fatalf("update status: %v", err)
		}
		if _, err := inv.InviteByEmail(ctx, fx.email, fx.inviter); !errors.Is(err, ErrIdentityNotInvitable) {
			t.Fatalf("%s: err = %v, quero ErrIdentityNotInvitable", status, err)
		}
	}
}

// Aceite: só a identidade convidada ativa o membership; o aceite carimba
// activated_at; membership já resolvido não re-ativa.
func TestAcceptInvitation(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "accept")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	m, err := inv.InviteByEmail(ctx, fx.email, fx.inviter)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}

	// Outra identidade não aceita convite alheio.
	if _, err := inv.Accept(ctx, m.ID, fx.inviter); !errors.Is(err, ErrNotInviteOwner) {
		t.Fatalf("aceite alheio: err = %v, quero ErrNotInviteOwner", err)
	}

	got, err := inv.Accept(ctx, m.ID, fx.identity.ID)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if got.Status != domain.MembershipActive {
		t.Fatalf("status pós-aceite = %s, quero active", got.Status)
	}
	var activatedSet bool
	if err := pool.QueryRow(ctx,
		"SELECT activated_at IS NOT NULL FROM membership WHERE id = $1", m.ID.String()).Scan(&activatedSet); err != nil {
		t.Fatalf("consulta activated_at: %v", err)
	}
	if !activatedSet {
		t.Fatalf("aceite deveria carimbar activated_at")
	}

	// Re-aceite: a transição de domínio recusa (não está mais invited).
	if _, err := inv.Accept(ctx, m.ID, fx.identity.ID); !errors.Is(err, domain.ErrMembershipTransition) {
		t.Fatalf("re-aceite: err = %v, quero ErrMembershipTransition", err)
	}
}

// Fail-closed: uma identidade suspensa NÃO ativa um convite pendente — a
// suspensão deixa memberships invited intactos, então sem a checagem de status
// no Accept ela ganharia um tenant novo enquanto suspensa.
func TestAcceptRejectsSuspendedIdentity(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "accsusp")
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, nil)

	m, err := inv.InviteByEmail(ctx, fx.email, fx.inviter)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	// A identidade é suspensa DEPOIS do convite (o membership invited sobrevive).
	if _, err := pool.Exec(ctx,
		"UPDATE identity SET status = 'suspended' WHERE id = $1", fx.identity.ID.String()); err != nil {
		t.Fatalf("suspende identidade: %v", err)
	}
	if _, err := inv.Accept(ctx, m.ID, fx.identity.ID); !errors.Is(err, ErrIdentityNotInvitable) {
		t.Fatalf("aceite de identidade suspensa: err = %v, quero ErrIdentityNotInvitable", err)
	}
	// O membership continua invited — não foi ativado.
	if got := membershipStatus(t, pool, m.ID); got != "invited" {
		t.Fatalf("membership = %s, deveria continuar invited", got)
	}
}

// Barreira 1: o store tenant-scoped não escreve membership de outra organização
// e não enxerga membership de outro tenant.
func TestTenantMembershipStoreCrossTenantGuards(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeInviteFixture(t, pool, "cross")
	repo := NewTenantRepository(pool, fx.scopeB)

	// Escrita: membership do org A por um store escopado em B.
	foreign, err := domain.NewMembership(fx.identity.ID, fx.orgA)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewTenantMembershipStore(ttx).Create(ctx, foreign)
	}); !errors.Is(err, ErrCrossTenantWrite) {
		t.Fatalf("escrita cross-tenant: err = %v, quero ErrCrossTenantWrite", err)
	}

	// Leitura: o membership em A é invisível para o store de B.
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		_, e := NewTenantMembershipStore(ttx).Get(ctx, fx.memA.ID)
		return e
	}); !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("leitura cross-tenant: err = %v, quero ErrMembershipNotFound", err)
	}
}
