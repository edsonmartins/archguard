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
	"time"

	"github.com/casdoor/casdoor/internal/adapters/alerting"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Renovação normal seguida de reuso: a rotação invalida o token anterior; o
// reuso do anterior revoga a FAMÍLIA + evento severidade alta + alerta crítico
// (cenários "Renovação normal" e "Reuso detectado").
func TestRefreshExchangeRotationAndReuse(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "refresh")
	cleanupAudit(t, pool, fx.orgA)

	// Sessão ativa para a FK do refresh token.
	sessID := uuid.New()
	at := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx,
		`INSERT INTO auth_session (id, identity_id, membership_id, organization_id, status, proven_aal, token_generation, auth_time)
		 VALUES ($1,$2,$3,$4,'active','aal2',1,$5)`,
		sessID.String(), fx.identity.ID.String(), fx.memA.ID.String(), fx.orgA.String(), at); err != nil {
		t.Fatalf("insere sessão: %v", err)
	}

	repo := NewTenantRepository(pool, fx.tenantScopeA)
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	// Primeiro refresh da família.
	secret1, hash1, _ := domain.NewRefreshSecret()
	first, err := domain.NewRefreshFamily(sessID, fx.orgA, hash1, exp)
	if err != nil {
		t.Fatalf("NewRefreshFamily: %v", err)
	}
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewRefreshTokenStore(ttx).Create(ctx, first)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	alerter := alerting.NewMemoryAlerter()
	x := NewRefreshExchanger(repo, NewAuditWriter(pool, fixedClock()), alerter)

	// Renovação normal: secret1 -> secret2; secret1 fica rotated.
	res, err := x.Exchange(ctx, secret1, exp)
	if err != nil {
		t.Fatalf("Exchange normal: %v", err)
	}
	if res.NewSecret == secret1 || res.NewToken.FamilyID != first.FamilyID {
		t.Fatalf("a renovação deveria dar um sucessor na mesma família: %+v", res)
	}

	// Reuso: apresentar secret1 (rotated) de novo revoga a família.
	if _, err := x.Exchange(ctx, secret1, exp); !errors.Is(err, domain.ErrRefreshReuse) {
		t.Fatalf("reuso deveria ser detectado: %v", err)
	}

	// Toda a família revogada (o sucessor também).
	var active int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM refresh_token WHERE family_id = $1 AND status <> 'revoked'",
		first.FamilyID.String()).Scan(&active); err != nil {
		t.Fatalf("contagem: %v", err)
	}
	if active != 0 {
		t.Fatalf("a família inteira deveria estar revogada, restam %d ativos/rotated", active)
	}
	// Evento de severidade alta auditado.
	if countAction(t, pool, fx.orgA, domain.ActionRefreshReuse) != 1 {
		t.Fatalf("o reuso deveria ter sido auditado")
	}
	// Alerta crítico emitido.
	alerts := alerter.Alerts()
	if len(alerts) != 1 || alerts[0].Severity != domain.SeverityCritical {
		t.Fatalf("deveria ter emitido um alerta crítico, veio %+v", alerts)
	}

	// O sucessor (secret2) também não vale mais.
	if _, err := x.Exchange(ctx, res.NewSecret, exp); !errors.Is(err, domain.ErrRefreshReuse) {
		t.Fatalf("o sucessor revogado deveria sinalizar reuso ao ser apresentado: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM refresh_token WHERE family_id = $1", first.FamilyID.String())
		_, _ = pool.Exec(bg, "DELETE FROM auth_session WHERE id = $1", sessID.String())
	})
}
