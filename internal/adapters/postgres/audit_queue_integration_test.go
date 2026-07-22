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
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// A fila assíncrona: eventos não privilegiados são enfileirados (sem tocar a
// cadeia) e um drainer os sela na cadeia, em ordem, verificáveis.
func TestAuditQueueEnqueueAndDrain(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_event_queue WHERE organization_id = $1", org.String())
	})

	q := NewAuditQueue(fixedClock())
	for i := 0; i < 3; i++ {
		in := domain.AuditEventInput{
			OrganizationID: org,
			Action:         domain.ActionAuthLogin,
			Actor:          domain.AuditActor{IdentitySubject: "sub-queued"},
			Outcome:        domain.Allowed,
		}
		if err := q.Enqueue(ctx, pool, in); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	// Ainda não drenado: a cadeia está vazia, a fila cheia.
	if got := len(readChain(t, pool, org)); got != 0 {
		t.Fatalf("cadeia deveria estar vazia antes do drain, veio %d", got)
	}
	var queued int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_event_queue WHERE organization_id = $1", org.String()).Scan(&queued); err != nil {
		t.Fatalf("count fila: %v", err)
	}
	if queued != 3 {
		t.Fatalf("fila deveria ter 3, veio %d", queued)
	}

	// Drena: os 3 entram na cadeia, em ordem, e a fila esvazia.
	n, err := q.Drain(ctx, pool, 10)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if n != 3 {
		t.Fatalf("drenados = %d, quero 3", n)
	}
	verifyChain(t, pool, org, 3)
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_event_queue WHERE organization_id = $1", org.String()).Scan(&queued); err != nil {
		t.Fatalf("count fila pós-drain: %v", err)
	}
	if queued != 0 {
		t.Fatalf("fila deveria estar vazia após o drain, veio %d", queued)
	}

	// Drenar de novo com a fila vazia é no-op.
	if n, err := q.Drain(ctx, pool, 10); err != nil || n != 0 {
		t.Fatalf("re-drain vazio: n=%d err=%v", n, err)
	}
}

// A fila recusa ações privilegiadas (L3) — essas exigem o AuditSink síncrono.
func TestAuditQueueRefusesPrivileged(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	q := NewAuditQueue(fixedClock())
	in := domain.AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         domain.ActionPrivilegedSessionOpen,
		Actor:          domain.AuditActor{IdentitySubject: "sub"},
		Outcome:        domain.Allowed,
	}
	if err := q.Enqueue(ctx, pool, in); err == nil {
		t.Fatalf("ação privilegiada não deveria ser enfileirável")
	}
}

// Fecha a alça do 002: uma troca de tenant grava no session_event_outbox; o
// drainer sela um evento tenant.switch na cadeia com o subject do ator e marca
// a linha do outbox como publicada; re-drenar é no-op.
func TestSwitchOutboxDrainToChain(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "obdrain")
	cleanupAudit(t, pool, fx.orgB)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM session_event_outbox WHERE identity_id = $1", fx.identity.ID.String())
	})

	// Executa uma troca real (A → B): escreve a linha do outbox.
	sess := activeSessionInA(t, pool, fx)
	sw := NewTenantSwitcher(NewIdentityRepository(pool, fx.scopeIdn), staticAALPolicy{aal: domain.AAL1})
	if _, err := sw.Switch(ctx, sess.ID, fx.memB); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	drainer := NewSwitchOutboxDrainer()
	n, err := drainer.Drain(ctx, pool, 10)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if n != 1 {
		t.Fatalf("drenados = %d, quero 1", n)
	}

	// Um evento tenant.switch na cadeia de B, com o subject e a org corretos.
	var action, subject, orgID string
	if err := pool.QueryRow(ctx,
		`SELECT action, actor_subject, organization_id::text FROM audit_event
		 WHERE organization_id = $1 AND seq = 1`, fx.orgB.String()).
		Scan(&action, &subject, &orgID); err != nil {
		t.Fatalf("leitura do evento: %v", err)
	}
	if action != string(domain.ActionTenantSwitch) {
		t.Fatalf("action = %q, quero tenant.switch", action)
	}
	if subject != fx.identity.Subject {
		t.Fatalf("actor_subject = %q, quero o subject opaco da identidade", subject)
	}
	verifyChain(t, pool, fx.orgB, 1)

	// O outbox foi marcado publicado; re-drenar não repete.
	var published int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM session_event_outbox WHERE identity_id = $1 AND published_at IS NOT NULL",
		fx.identity.ID.String()).Scan(&published); err != nil {
		t.Fatalf("count publicado: %v", err)
	}
	if published != 1 {
		t.Fatalf("linha do outbox deveria estar publicada")
	}
	if n, err := drainer.Drain(ctx, pool, 10); err != nil || n != 0 {
		t.Fatalf("re-drain: n=%d err=%v, quero 0/nil", n, err)
	}
}
