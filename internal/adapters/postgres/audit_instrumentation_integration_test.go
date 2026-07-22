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
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// adminCtx returns a context carrying an acting principal (an admin), as the
// authenticated request boundary would set (T-017).
func adminCtx() context.Context {
	return domain.WithPrincipal(context.Background(),
		domain.AuditActor{IdentitySubject: "admin-subject"})
}

// countAction returns how many audit events of an action exist for an org.
func countAction(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, action domain.Action) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE organization_id = $1 AND action = $2",
		org.String(), string(action)).Scan(&n); err != nil {
		t.Fatalf("count action %s: %v", action, err)
	}
	return n
}

// A instrumentação: convite/aceite/revogação gravam o evento certo, com o ator
// do contexto, ATOMICAMENTE na transação da operação.
func TestInstrumentationMembershipEvents(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := adminCtx()
	fx := makeInviteFixture(t, pool, "instr")
	cleanupAudit(t, pool, fx.orgB)
	w := NewAuditWriter(pool, fixedClock())
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, w)

	m, err := inv.InviteByEmail(ctx, fx.email, fx.inviter)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if countAction(t, pool, fx.orgB, domain.ActionMembershipInvite) != 1 {
		t.Fatalf("membership.invite não gravado")
	}

	if _, err := inv.Accept(ctx, m.ID, fx.identity.ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if countAction(t, pool, fx.orgB, domain.ActionMembershipAccept) != 1 {
		t.Fatalf("membership.accept não gravado")
	}

	// A cadeia da org está íntegra (os eventos entraram encadeados).
	verifyChain(t, pool, fx.orgB, 2)
	// O ator gravado é o principal do contexto.
	var subject string
	if err := pool.QueryRow(context.Background(),
		"SELECT actor_subject FROM audit_event WHERE organization_id = $1 AND seq = 1", fx.orgB.String()).Scan(&subject); err != nil {
		t.Fatalf("leitura do ator: %v", err)
	}
	if subject != "admin-subject" {
		t.Fatalf("actor_subject = %q, quero admin-subject", subject)
	}
}

// Fail-closed: com emissor de auditoria mas SEM principal no contexto, a
// operação é recusada (ErrNoPrincipal) e nada é gravado nem no negócio.
func TestInstrumentationFailsClosedWithoutPrincipal(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeInviteFixture(t, pool, "noprinc")
	cleanupAudit(t, pool, fx.orgB)
	w := NewAuditWriter(pool, fixedClock())
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, w)

	// Contexto SEM principal.
	_, err := inv.InviteByEmail(context.Background(), fx.email, fx.inviter)
	if !errors.Is(err, domain.ErrNoPrincipal) {
		t.Fatalf("sem principal: err = %v, quero ErrNoPrincipal", err)
	}
	// Rollback atômico: o membership NÃO foi criado (a transação inteira desfez).
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM membership WHERE identity_id = $1 AND organization_id = $2",
		fx.identity.ID.String(), fx.orgB.String()).Scan(&n); err != nil {
		t.Fatalf("count membership: %v", err)
	}
	if n != 0 {
		t.Fatalf("operação sem auditoria deveria ter dado rollback, veio %d membership(s)", n)
	}
}

// Cascata de identidade: suspensão grava UM evento identity.suspend por org
// afetada, cada um na cadeia do seu tenant.
func TestInstrumentationIdentityCascadePerOrg(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := adminCtx()
	fx := makeSessionFixture(t, pool, "casc")
	cleanupAudit(t, pool, fx.orgA, fx.orgB)
	w := NewAuditWriter(pool, fixedClock())

	lifecycle := NewIdentityLifecycle(NewIdentityRepository(pool, fx.scopeIdn), w)
	if _, err := lifecycle.Suspend(ctx); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	// A identidade tem membership em A e B → um evento identity.suspend em cada.
	if countAction(t, pool, fx.orgA, domain.ActionIdentitySuspend) != 1 {
		t.Fatalf("identity.suspend não gravado na org A")
	}
	if countAction(t, pool, fx.orgB, domain.ActionIdentitySuspend) != 1 {
		t.Fatalf("identity.suspend não gravado na org B")
	}
	verifyChain(t, pool, fx.orgA, 1)
	verifyChain(t, pool, fx.orgB, 1)
}

// A exportação é auditada (audit.export) na cadeia da org exportada.
func TestInstrumentationExportAudited(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := adminCtx()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	w := NewAuditWriter(pool, fixedClock())
	appendN(t, w, org, 2)

	resolve := func(keyID string) ([]byte, bool) { pub := signer.PublicKey(keyID); return pub, pub != nil }
	exporter := NewTrailExporter(pool, w)
	if err := exporter.Export(ctx, org, resolve, fixedClock(), &bytes.Buffer{}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if countAction(t, pool, org, domain.ActionAuditExport) != 1 {
		t.Fatalf("audit.export não gravado")
	}
}
