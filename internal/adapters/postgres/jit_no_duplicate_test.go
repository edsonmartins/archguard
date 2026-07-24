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

// T-020 / spec "JIT provisioning com e-mail conhecido": um login federado cujo
// e-mail já é conhecido NÃO cria uma segunda identidade — através de protocolos
// (SAML e OIDC) e de fontes (SCIM antes, federação depois), a pessoa é UMA
// identidade em todos os tenants (RFC-0007 critério 1).
func TestJITKnownEmailNeverDuplicates(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	orgSCIM := makeTenant(t, pool, "jit-scim")
	orgSAML := makeTenant(t, pool, "jit-saml")
	orgOIDC := makeTenant(t, pool, "jit-oidc")
	prov := NewDirectoryProvisioner(pool, cust)

	email := "pessoa@cli.com"
	hash, _ := cust.HashEmail(email)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash)
		_, _ = pool.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", hash)
	})

	// 1) A pessoa entra primeiro por SCIM.
	idSCIM, err := prov.ProvisionUser(ctx, orgSCIM.orgID, domain.DirectorySyncRecord{Email: email, Active: true})
	if err != nil {
		t.Fatalf("SCIM: %v", err)
	}

	// 2) Depois faz login federado SAML (e-mail conhecido) em outro tenant.
	idSAML, err := prov.ProvisionFederated(ctx, orgSAML.orgID, domain.FederatedIdentity{
		Provider: "entra", Protocol: domain.FederationSAML, Email: email, DisplayName: "Pessoa"})
	if err != nil {
		t.Fatalf("SAML JIT: %v", err)
	}

	// 3) E login federado OIDC (variando case/espaço) em outro tenant.
	idOIDC, err := prov.ProvisionFederated(ctx, orgOIDC.orgID, domain.FederatedIdentity{
		Provider: "okta", Protocol: domain.FederationOIDC, Email: "  PESSOA@cli.com ", DisplayName: "Pessoa"})
	if err != nil {
		t.Fatalf("OIDC JIT: %v", err)
	}

	if idSCIM != idSAML || idSAML != idOIDC {
		t.Fatalf("JIT deveria reusar a MESMA identidade (%s/%s/%s)", idSCIM, idSAML, idOIDC)
	}

	// Exatamente UMA identidade, e um membership por tenant (3).
	var identCount, memCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity WHERE email_hash = $1", hash).Scan(&identCount); err != nil {
		t.Fatalf("count id: %v", err)
	}
	if identCount != 1 {
		t.Fatalf("federação por 2 protocolos NÃO deveria duplicar identidade, veio %d", identCount)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM membership WHERE identity_id IN (SELECT id FROM identity WHERE email_hash = $1)", hash).Scan(&memCount); err != nil {
		t.Fatalf("count mem: %v", err)
	}
	if memCount != 3 {
		t.Fatalf("deveria haver 3 memberships (SCIM+SAML+OIDC), veio %d", memCount)
	}
}
