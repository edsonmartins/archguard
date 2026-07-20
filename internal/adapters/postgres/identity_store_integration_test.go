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
	"os"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupIdentityStore stands up a live store against ARCHGUARD_TEST_DSN, applying
// the real migrations first (seeding the legacy organization table the way Sync2
// would, so 0003/0004 apply). It skips when the DSN is unset.
func setupIdentityStore(t *testing.T) (*IdentityStore, *keycustodian.Provisional) {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — pulando teste de integração do IdentityStore")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS organization (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cust, err := keycustodian.NewProvisional(key)
	if err != nil {
		t.Fatalf("custodian: %v", err)
	}
	return NewIdentityStore(pool), cust
}

func newIdentityWithEmail(t *testing.T, cust domain.KeyCustodian, email string) domain.Identity {
	t.Helper()
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	hash, err := cust.HashEmail(email)
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	idn.EmailHash = hash
	idn.PrimaryEmailEnc = []byte("ciphertext-placeholder")
	return idn
}

func TestIdentityStoreCreateAndLoginByHash(t *testing.T) {
	store, cust := setupIdentityStore(t)
	ctx := context.Background()

	idn := newIdentityWithEmail(t, cust, "Alice+Ops@Empresa.com")
	if err := store.Create(ctx, idn); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { cleanupIdentity(store, idn.EmailHash) })

	// Login por hash: e-mail com case/espaços diferentes encontra a mesma
	// identidade (a normalização + HMAC casam).
	got, err := store.FindByEmail(ctx, cust, "  alice+ops@empresa.COM ")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != idn.ID || got.Subject != idn.Subject {
		t.Errorf("FindByEmail devolveu identidade errada: %v/%s", got.ID, got.Subject)
	}
	if got.Status != domain.IdentityActive || got.Type != domain.IdentityHuman {
		t.Errorf("campos não recompuseram: status=%q type=%q", got.Status, got.Type)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at deveria ter sido preenchido pelo banco")
	}
}

func TestIdentityStoreEmailHashIsUnique(t *testing.T) {
	store, cust := setupIdentityStore(t)
	ctx := context.Background()

	a := newIdentityWithEmail(t, cust, "dup@empresa.com")
	if err := store.Create(ctx, a); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	t.Cleanup(func() { cleanupIdentity(store, a.EmailHash) })

	// Segunda identidade com o MESMO e-mail (mesmo hash) deve violar o índice
	// único parcial (migration 0005).
	b := newIdentityWithEmail(t, cust, "DUP@empresa.com")
	if err := store.Create(ctx, b); err == nil {
		cleanupIdentity(store, b.EmailHash)
		t.Error("email_hash duplicado deveria violar a unicidade")
	}
}

func TestIdentityStoreNotFound(t *testing.T) {
	store, cust := setupIdentityStore(t)
	ctx := context.Background()
	if _, err := store.FindByEmail(ctx, cust, "ninguem@empresa.com"); !errors.Is(err, ErrIdentityNotFound) {
		t.Errorf("erro = %v, quer ErrIdentityNotFound", err)
	}
}

func TestIdentityStoreNilEmailHashDoesNotCollide(t *testing.T) {
	store, _ := setupIdentityStore(t)
	ctx := context.Background()

	// Duas contas de serviço sem e-mail (email_hash nulo) coexistem — o índice
	// único é PARCIAL (WHERE email_hash IS NOT NULL).
	var subjects []string
	for i := 0; i < 2; i++ {
		idn, err := domain.NewIdentity(domain.IdentityService)
		if err != nil {
			t.Fatalf("NewIdentity: %v", err)
		}
		if err := store.Create(ctx, idn); err != nil {
			t.Fatalf("Create serviço %d (email_hash nulo não deveria colidir): %v", i, err)
		}
		subjects = append(subjects, idn.Subject)
	}
	t.Cleanup(func() {
		for _, s := range subjects {
			_, _ = store.db.Exec(context.Background(), "DELETE FROM identity WHERE subject = $1", s)
		}
	})
}

func cleanupIdentity(store *IdentityStore, emailHash []byte) {
	_, _ = store.db.Exec(context.Background(), "DELETE FROM identity WHERE email_hash = $1", emailHash)
}
