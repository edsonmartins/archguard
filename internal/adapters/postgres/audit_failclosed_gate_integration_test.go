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
	"github.com/jackc/pgx/v5"
)

// downEmitter simulates an unavailable audit subsystem: every append fails.
type downEmitter struct{}

func (downEmitter) AppendTx(_ context.Context, _ pgx.Tx, _ domain.AuditEventInput) (domain.SealedEvent, error) {
	return domain.SealedEvent{}, errors.New("auditoria indisponível")
}

// TestFailClosedGate is the package gate for fail-closed (I-5.4, RFC-0003 §7):
// when the audit subsystem is UNAVAILABLE, a privileged/administrative operation
// is DENIED and rolled back — never completed unaudited. The audit write shares
// the operation's transaction (T-017), so a failing audit rolls the whole
// operation back.
func TestFailClosedGate(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := domain.WithPrincipal(context.Background(), domain.AuditActor{IdentitySubject: "admin"})
	fx := makeInviteFixture(t, pool, "failclosed")

	// Inviter com auditoria INDISPONÍVEL.
	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, downEmitter{})

	_, err := inv.InviteByEmail(ctx, fx.email, fx.inviter)
	if err == nil {
		t.Fatalf("com auditoria indisponível, o convite deveria ser NEGADO")
	}

	// Fail-closed: nada foi persistido — a operação inteira deu rollback.
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM membership WHERE identity_id = $1 AND organization_id = $2",
		fx.identity.ID.String(), fx.orgB.String()).Scan(&n); err != nil {
		t.Fatalf("count membership: %v", err)
	}
	if n != 0 {
		t.Fatalf("operação com auditoria indisponível deveria ter dado rollback, veio %d membership(s)", n)
	}

	// Também a revogação (mutação privilegiada) é negada quando a auditoria cai.
	sfx := makeSessionFixture(t, pool, "failclosed2")
	seedTwoTenantSessions(t, pool, sfx)
	revoker := NewMembershipRevoker(NewTenantRepository(pool, sfx.tenantScopeA), downEmitter{})
	if _, _, err := revoker.RevokeMembership(ctx, sfx.memA.ID); err == nil {
		t.Fatalf("revogação com auditoria indisponível deveria ser NEGADA")
	}
	// A membership NÃO foi revogada (rollback).
	if got := membershipStatus(t, pool, sfx.memA.ID); got != "active" {
		t.Fatalf("membership deveria continuar active após rollback, veio %s", got)
	}
}

// Sanidade: a mesma operação, com auditoria DISPONÍVEL, conclui — provando que
// a negação acima é causada pela indisponibilidade, não por outra coisa.
func TestFailClosedGateAvailableSucceeds(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := domain.WithPrincipal(context.Background(), domain.AuditActor{IdentitySubject: "admin"})
	fx := makeInviteFixture(t, pool, "avail")
	cleanupAudit(t, pool, fx.orgB)

	inv := NewInviter(NewTenantRepository(pool, fx.scopeB), fx.custodian, NewAuditWriter(pool, fixedClock()))
	if _, err := inv.InviteByEmail(ctx, fx.email, fx.inviter); err != nil {
		t.Fatalf("com auditoria disponível o convite deveria concluir: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM membership WHERE identity_id = $1 AND organization_id = $2",
		fx.identity.ID.String(), fx.orgB.String()).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("membership deveria ter sido criado, veio %d", n)
	}
}
