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
)

// TestAccessAuditorRecordsDurably: o auditor durável grava o acesso global na tabela
// append-only (migração 0035). Cobre o caminho self (login/console).
func TestAccessAuditorRecordsDurably(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	a := NewAccessAuditor(pool)
	access := domain.GlobalAccess{
		Principal: "sub-" + uniqueSuffix(),
		Reason:    "login: resolução de memberships da própria identidade",
		Scope:     domain.ScopeSelf,
	}
	if err := a.Record(ctx, access); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var scope, reason string
	if err := pool.QueryRow(ctx,
		"SELECT scope, reason FROM global_access_audit WHERE principal = $1", access.Principal,
	).Scan(&scope, &reason); err != nil {
		t.Fatalf("linha não encontrada: %v", err)
	}
	if scope != "self" || reason != access.Reason {
		t.Errorf("gravado scope=%q reason=%q, esperado self/%q", scope, reason, access.Reason)
	}

	// Acesso malformado não é gravado.
	if err := a.Record(ctx, domain.GlobalAccess{Scope: domain.ScopeSelf}); err == nil {
		t.Error("acesso sem principal/motivo deveria ser rejeitado")
	}
}

// TestAccessAuditorAppendOnly: a trilha é append-only — UPDATE e DELETE são abortados
// no banco pelo trigger (defesa em profundidade do INV-2, migração 0035).
func TestAccessAuditorAppendOnly(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	principal := "sub-" + uniqueSuffix()
	if err := NewAccessAuditor(pool).Record(ctx, domain.GlobalAccess{
		Principal: principal, Reason: "auditoria", Scope: domain.ScopeCrossTenant,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE global_access_audit SET reason = 'x' WHERE principal = $1", principal); err == nil {
		t.Error("UPDATE deveria ser bloqueado (append-only)")
	}
	if _, err := pool.Exec(ctx, "DELETE FROM global_access_audit WHERE principal = $1", principal); err == nil {
		t.Error("DELETE deveria ser bloqueado (append-only)")
	}
}
