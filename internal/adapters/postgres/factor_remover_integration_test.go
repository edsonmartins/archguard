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
)

// Cenário "Remoção de fator": remover um fator forte grava um evento
// factor.remove com ator (o principal do contexto), alvo (o subject da
// identidade afetada) e resultado, ATOMICAMENTE na transação da remoção.
func TestFactorRemoverAuditsRemoval(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "factorrm")
	cleanupAudit(t, pool, fx.orgA)

	// A identidade tem um fator forte (WebAuthn).
	cred, err := domain.NewWebAuthnCredential(fx.identity.ID, []byte("public-key-material"))
	if err != nil {
		t.Fatalf("NewWebAuthnCredential: %v", err)
	}
	if err := NewCredentialStore(pool).Create(context.Background(), cred); err != nil {
		t.Fatalf("cria credencial: %v", err)
	}

	ctx := adminCtx()
	remover := NewFactorRemover(NewTenantRepository(pool, fx.tenantScopeA), NewAuditWriter(pool, fixedClock()))
	if err := remover.RemoveStrongFactor(ctx, fx.identity.ID, cred.ID, fx.identity.Subject); err != nil {
		t.Fatalf("RemoveStrongFactor: %v", err)
	}

	// A credencial sumiu.
	remaining, err := NewCredentialStore(pool).ListByIdentity(context.Background(), fx.identity.ID)
	if err != nil {
		t.Fatalf("ListByIdentity: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("o fator deveria ter sido removido, restam %d", len(remaining))
	}

	// O evento factor.remove foi gravado com o ator do contexto.
	if countAction(t, pool, fx.orgA, domain.ActionFactorRemove) != 1 {
		t.Fatalf("factor.remove não gravado")
	}
	var subject, targetID string
	if err := pool.QueryRow(context.Background(),
		"SELECT actor_subject, target_id FROM audit_event WHERE organization_id = $1 AND action = $2",
		fx.orgA.String(), string(domain.ActionFactorRemove)).Scan(&subject, &targetID); err != nil {
		t.Fatalf("leitura do evento: %v", err)
	}
	if subject != "admin-subject" {
		t.Fatalf("ator = %q, quero admin-subject", subject)
	}
	if targetID != fx.identity.Subject {
		t.Fatalf("alvo = %q, quero o subject da identidade afetada", targetID)
	}
}

// Fail-closed: sem principal no contexto, a remoção é recusada (ErrNoPrincipal)
// e o fator NÃO é removido — remoção não auditável não acontece (I-5.4).
func TestFactorRemoverFailsClosedWithoutPrincipal(t *testing.T) {
	pool := setupTenantPool(t)
	fx := makeSessionFixture(t, pool, "factorrmnp")
	cleanupAudit(t, pool, fx.orgA)

	cred, _ := domain.NewWebAuthnCredential(fx.identity.ID, []byte("public-key"))
	if err := NewCredentialStore(pool).Create(context.Background(), cred); err != nil {
		t.Fatalf("cria credencial: %v", err)
	}

	remover := NewFactorRemover(NewTenantRepository(pool, fx.tenantScopeA), NewAuditWriter(pool, fixedClock()))
	// Contexto SEM principal.
	if err := remover.RemoveStrongFactor(context.Background(), fx.identity.ID, cred.ID, fx.identity.Subject); !errors.Is(err, domain.ErrNoPrincipal) {
		t.Fatalf("sem principal: err = %v, quero ErrNoPrincipal", err)
	}
	// Rollback atômico: o fator continua lá.
	remaining, _ := NewCredentialStore(pool).ListByIdentity(context.Background(), fx.identity.ID)
	if len(remaining) != 1 {
		t.Fatalf("a remoção não auditável deveria ter sido desfeita, restam %d", len(remaining))
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM credential WHERE identity_id = $1", fx.identity.ID.String())
	})
}
