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
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

// fakeSecretStore is an in-memory domain.SecretStore that records deletions, so a
// test can assert the compensation path.
type fakeSecretStore struct {
	mu      sync.Mutex
	seq     int
	secrets map[string][]byte
	deleted []string
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{secrets: map[string][]byte{}}
}

func (f *fakeSecretStore) Put(_ context.Context, secret []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	ref := fmt.Sprintf("vault://fake/%d", f.seq)
	f.secrets[ref] = append([]byte(nil), secret...)
	return ref, nil
}

func (f *fakeSecretStore) Get(_ context.Context, ref string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.secrets[ref]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return append([]byte(nil), s...), nil
}

func (f *fakeSecretStore) Delete(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.secrets, ref)
	f.deleted = append(f.deleted, ref)
	return nil
}

// A credencial de bind é custodiada no cofre; só a referência vai ao banco (o
// segredo nunca aparece em coluna alguma, INV-7). Resolve devolve o segredo.
func TestDirectoryConnectorProvisionCustodiesSecret(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeTenant(t, pool, "prov")
	repo := NewTenantRepository(pool, fx.scope)
	vault := newFakeSecretStore()
	prov := NewDirectoryConnectorProvisioner(repo, vault)

	bindSecret := []byte("s3nh4-de-bind-super-secreta")
	conn, err := prov.Provision(ctx, ConnectorSpec{
		OrganizationID: fx.orgID, Kind: domain.DirectoryAD, Name: "AD Corp",
		ScopeFilter: "(objectClass=user)",
		Attributes:  []domain.AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}},
	}, bindSecret)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	// A coluna credential_ref guarda a referência, NUNCA o segredo.
	var credRef string
	if err := pool.QueryRow(ctx, "SELECT credential_ref FROM directory_connector WHERE id = $1", conn.ID.String()).Scan(&credRef); err != nil {
		t.Fatalf("leitura da coluna: %v", err)
	}
	if credRef != conn.CredentialRef {
		t.Fatalf("ref persistida diverge: %q vs %q", credRef, conn.CredentialRef)
	}
	if bytes.Contains([]byte(credRef), bindSecret) {
		t.Fatalf("o segredo NÃO deveria aparecer na coluna do banco")
	}

	// Resolve devolve o segredo original (para o syncer).
	got, err := prov.ResolveCredential(ctx, conn)
	if err != nil {
		t.Fatalf("ResolveCredential: %v", err)
	}
	if !bytes.Equal(got, bindSecret) {
		t.Fatalf("credencial resolvida diverge do original")
	}
}

// Falha ao persistir (nome duplicado) compensa a entrada órfã no cofre: um
// segredo cofrado nunca sobrevive à linha que deveria apontá-lo (RFC-0004 §4).
func TestDirectoryConnectorProvisionCompensatesOnFailure(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeTenant(t, pool, "provcomp")
	repo := NewTenantRepository(pool, fx.scope)
	vault := newFakeSecretStore()
	prov := NewDirectoryConnectorProvisioner(repo, vault)

	spec := ConnectorSpec{
		OrganizationID: fx.orgID, Kind: domain.DirectoryAD, Name: "Duplicado",
		ScopeFilter: "(objectClass=user)",
		Attributes:  []domain.AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}},
	}
	if _, err := prov.Provision(ctx, spec, []byte("primeiro")); err != nil {
		t.Fatalf("primeira provisão: %v", err)
	}
	// Segundo com o MESMO nome viola UNIQUE(org, name) → falha na persistência.
	if _, err := prov.Provision(ctx, spec, []byte("segundo")); err == nil {
		t.Fatalf("provisão duplicada deveria falhar")
	}
	if len(vault.deleted) != 1 {
		t.Fatalf("a entrada órfã do segundo segredo deveria ter sido compensada, deletados=%v", vault.deleted)
	}
}
