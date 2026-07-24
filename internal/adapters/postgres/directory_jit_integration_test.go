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

// JIT com e-mail conhecido NÃO cria segunda identidade (spec "JIT provisioning com
// e-mail conhecido") e reativa um membership suspenso ("cria ou ativa").
func TestDirectoryJITReuseAndReactivate(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	org := makeTenant(t, pool, "jit")
	prov := NewDirectoryProvisioner(pool, cust)

	email := "ana@cli.com"
	hash, _ := cust.HashEmail(email)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash)
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", hash)
	})

	// Pré-condição: a pessoa já existe (provisionada por SCIM/LDAP antes).
	pre, err := prov.ProvisionUser(ctx, org.orgID, domain.DirectorySyncRecord{Email: email, Active: true})
	if err != nil {
		t.Fatalf("provisão prévia: %v", err)
	}

	// Login federado com o MESMO e-mail: não cria nova identidade.
	fed := domain.FederatedIdentity{Provider: "entra", Protocol: domain.FederationSAML, Email: email, DisplayName: "Ana"}
	jit, err := prov.ProvisionFederated(ctx, org.orgID, fed)
	if err != nil {
		t.Fatalf("ProvisionFederated: %v", err)
	}
	if jit != pre {
		t.Fatalf("JIT deveria reusar a identidade existente (%s != %s)", jit, pre)
	}
	var identCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", hash).Scan(&identCount); err != nil {
		t.Fatalf("count identidades: %v", err)
	}
	if identCount != 1 {
		t.Fatalf("JIT NÃO deveria duplicar identidade, veio %d", identCount)
	}

	// Suspende o membership (como se o diretório o tivesse desativado)...
	if _, err := pool.Exec(ctx, "UPDATE membership SET status = 'suspended' WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash); err != nil {
		t.Fatalf("suspende: %v", err)
	}
	// ...e um novo login federado REATIVA (cria ou ativa).
	if _, err := prov.ProvisionFederated(ctx, org.orgID, fed); err != nil {
		t.Fatalf("JIT reativação: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "active" {
		t.Fatalf("membership suspenso deveria ter sido reativado pelo JIT, veio %s", status)
	}
}

// JIT sem e-mail é rejeitado (sem chave de dedup).
func TestDirectoryJITRequiresEmail(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := makeTenant(t, pool, "jit-noemail")
	prov := NewDirectoryProvisioner(pool, testCustodian(t))
	if _, err := prov.ProvisionFederated(ctx, org.orgID, domain.FederatedIdentity{Provider: "x"}); err == nil {
		t.Fatalf("JIT sem e-mail deveria falhar")
	}
}
