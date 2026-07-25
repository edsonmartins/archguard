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

package boot

import (
	"bytes"
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
)

type fakeIdentityStore struct {
	existing *domain.Identity // if set, FindByEmailHash returns it
	created  *domain.Identity // captures the Create argument
}

func (f *fakeIdentityStore) FindByEmailHash(_ context.Context, _ []byte) (domain.Identity, error) {
	if f.existing != nil {
		return *f.existing, nil
	}
	return domain.Identity{}, postgres.ErrIdentityNotFound
}

func (f *fakeIdentityStore) Create(_ context.Context, idn domain.Identity) error {
	f.created = &idn
	return nil
}

// TestProvisionAdminIdentityReusesExisting is the dedup guarantee (RFC-0002): an
// identity already matching the e-mail hash is reused, never duplicated.
func TestProvisionAdminIdentityReusesExisting(t *testing.T) {
	existing, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	existing.EmailHash = []byte("hash-abc")
	store := &fakeIdentityStore{existing: &existing}

	idn, created, err := provisionAdminIdentity(context.Background(), store, []byte("hash-abc"))
	if err != nil {
		t.Fatalf("provisionAdminIdentity: %v", err)
	}
	if created {
		t.Fatalf("must NOT create when an identity already exists (dedup)")
	}
	if store.created != nil {
		t.Fatalf("Create must not be called when reusing an existing identity")
	}
	if idn.ID != existing.ID {
		t.Fatalf("must return the existing identity")
	}
}

func TestProvisionAdminIdentityCreatesWhenAbsent(t *testing.T) {
	store := &fakeIdentityStore{} // FindByEmailHash => ErrIdentityNotFound
	hash := []byte("hash-xyz")

	idn, created, err := provisionAdminIdentity(context.Background(), store, hash)
	if err != nil {
		t.Fatalf("provisionAdminIdentity: %v", err)
	}
	if !created {
		t.Fatalf("must create a new identity when none matches")
	}
	if store.created == nil {
		t.Fatalf("Create should have been called")
	}
	if !bytes.Equal(idn.EmailHash, hash) {
		t.Fatalf("created identity must carry the e-mail hash (dedup key)")
	}
	if idn.Type != domain.IdentityHuman {
		t.Fatalf("admin identity must be human, got %v", idn.Type)
	}
	if !bytes.Equal(store.created.EmailHash, hash) {
		t.Fatalf("persisted identity must carry the e-mail hash")
	}
}
