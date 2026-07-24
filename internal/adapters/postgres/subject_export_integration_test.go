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
	"github.com/jackc/pgx/v5/pgxpool"
)

// subjectCipherCustodian is the union the export test needs from the custodian.
type subjectCipherCustodian interface {
	domain.KeyCustodian
	domain.SubjectCipher
}

// seedExportableIdentity creates an identity with encrypted personal fields + a
// membership in each of the two organizations.
func seedExportableIdentity(t *testing.T, p *pgxpool.Pool, a, b tenantFixture, cust subjectCipherCustodian) domain.Identity {
	t.Helper()
	ctx := context.Background()
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	idn.EmailHash, _ = cust.HashEmail("ana@cli.com")
	idn.PrimaryEmailEnc, _ = cust.EncryptForSubject(idn.Subject, []byte("ana@cli.com"))
	idn.DisplayNameEnc, _ = cust.EncryptForSubject(idn.Subject, []byte("Ana Souza"))
	if err := NewIdentityStore(p).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	for _, org := range []tenantFixture{a, b} {
		m, _ := domain.NewMembership(idn.ID, org.orgID)
		if err := NewTenantRepository(p, org.scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
			return NewTenantMembershipStore(ttx).Create(ctx, m)
		}); err != nil {
			t.Fatalf("cria membership: %v", err)
		}
	}
	return idn
}

// T-024 / spec "Requisição de acesso": a exportação de um titular numa organização
// contém apenas dados daquela organização e da identidade global — NUNCA dados de
// outra organização.
func TestSubjectExportIsolatesTenant(t *testing.T) {
	p := setupTenantPool(t)
	ctx := context.Background()
	cust := testCustodian(t)
	a := makeTenant(t, p, "exp-a")
	b := makeTenant(t, p, "exp-b")

	idn := seedExportableIdentity(t, p, a, b, cust)
	t.Cleanup(func() {
		_, _ = p.Exec(context.Background(), "DELETE FROM membership WHERE identity_id = $1", idn.ID.String())
		_, _ = p.Exec(context.Background(), "DELETE FROM identity WHERE id = $1", idn.ID.String())
	})

	exporter := NewSubjectExporter(p, cust)

	// Export para A: identidade global (e-mail decifrado) + membership de A APENAS.
	docA, err := exporter.Export(ctx, idn.Subject, a.orgID)
	if err != nil {
		t.Fatalf("Export A: %v", err)
	}
	if docA.Identity.Email != "ana@cli.com" || docA.Identity.DisplayName != "Ana Souza" {
		t.Fatalf("campos pessoais deveriam ser decifrados: %+v", docA.Identity)
	}
	if docA.Organization.OrganizationID != a.orgID.String() {
		t.Fatalf("export de A deveria conter o membership de A, veio %s", docA.Organization.OrganizationID)
	}
	if docA.Organization.OrganizationID == b.orgID.String() {
		t.Fatalf("export de A NÃO deveria conter dados de B")
	}

	// Export para B: membership de B, não de A.
	docB, err := exporter.Export(ctx, idn.Subject, b.orgID)
	if err != nil {
		t.Fatalf("Export B: %v", err)
	}
	if docB.Organization.OrganizationID != b.orgID.String() {
		t.Fatalf("export de B deveria conter o membership de B, veio %s", docB.Organization.OrganizationID)
	}
}
