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

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/domain"
)

func testCustodian(t *testing.T) *keycustodian.Provisional {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 7)
	}
	cust, err := keycustodian.NewProvisional(key)
	if err != nil {
		t.Fatalf("custodian: %v", err)
	}
	return cust
}

// SCIM/LDAP dedup por email_hash: um e-mail conhecido NUNCA cria segunda
// identidade (spec "Usuário já existente" / "JIT e-mail conhecido"); só ganha
// membership. E-mails diferindo só em case/espaço são a mesma identidade.
func TestDirectoryProvisionerDedupByEmailHash(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	orgA := makeTenant(t, pool, "prov-dedup-a")
	orgB := makeTenant(t, pool, "prov-dedup-b")

	prov := NewDirectoryProvisioner(pool, cust)
	hash, _ := cust.HashEmail("Ana@Cli.com")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash)
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", hash)
	})

	rec := domain.DirectorySyncRecord{Email: "Ana@Cli.com", Active: true}

	// 1) Primeira provisão em A: cria identidade + membership.
	id1, err := prov.ProvisionUser(ctx, orgA.orgID, rec)
	if err != nil {
		t.Fatalf("provisão 1: %v", err)
	}

	// 2) Mesma pessoa (case/espaço diferentes) em B: MESMA identidade, novo membership.
	id2, err := prov.ProvisionUser(ctx, orgB.orgID, domain.DirectorySyncRecord{Email: "  ana@cli.COM ", Active: true})
	if err != nil {
		t.Fatalf("provisão 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("dedup falhou: e-mail conhecido gerou identidade diferente (%s vs %s)", id1, id2)
	}

	// 3) Re-provisão idempotente em A: nenhum erro, mesma identidade.
	id3, err := prov.ProvisionUser(ctx, orgA.orgID, rec)
	if err != nil {
		t.Fatalf("provisão 3 (idempotente): %v", err)
	}
	if id3 != id1 {
		t.Fatalf("re-provisão deveria manter a identidade")
	}

	// Exatamente UMA identidade para o e-mail, e dois memberships (A e B).
	var identCount, memCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", hash).Scan(&identCount); err != nil {
		t.Fatalf("count identidades: %v", err)
	}
	if identCount != 1 {
		t.Fatalf("deveria haver exatamente 1 identidade, veio %d", identCount)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash).Scan(&memCount); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memCount != 2 {
		t.Fatalf("deveria haver 2 memberships (A e B), veio %d", memCount)
	}
}
