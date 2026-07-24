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

// T-021 / spec "Grupo de diretório sem mapeamento aprovado": provisionar um usuário
// vindo do diretório (mesmo membro de grupos) NÃO cria atribuição de papel — papéis
// privilegiados são sempre do ArchGuard, nunca auto-derivados do sync.
func TestDirectoryProvisionGrantsNoPrivilegedRole(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	org := makeTenant(t, pool, "no-priv")
	prov := NewDirectoryProvisioner(pool, cust)

	email := "operador@cli.com"
	hash, _ := cust.HashEmail(email)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM role_assignment WHERE membership_id IN (SELECT id FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1))", hash)
		_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash)
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", hash)
	})

	// O usuário do diretório é membro de grupos "privilegiados" na origem.
	rec := domain.DirectorySyncRecord{Email: email, Groups: []string{"CN=DBA", "CN=Admins"}, Active: true}
	if _, err := prov.ProvisionUser(ctx, org.orgID, rec); err != nil {
		t.Fatalf("provisão: %v", err)
	}

	// NENHUMA atribuição de papel foi criada pelo sync (papel privilegiado é do
	// ArchGuard, com mapeamento aprovado + ação explícita — nunca automático).
	var roleCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM role_assignment
		WHERE membership_id IN (
			SELECT id FROM membership WHERE identity_id IN (
				SELECT id FROM identity WHERE email_hash = $1))`, hash).Scan(&roleCount); err != nil {
		t.Fatalf("count role_assignment: %v", err)
	}
	if roleCount != 0 {
		t.Fatalf("o sync NÃO deveria conceder papel algum, veio %d atribuições", roleCount)
	}
}
