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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupCredentialStore(t *testing.T) (*pgxpool.Pool, *CredentialStore, domain.Identity) {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — pulando teste de integração do CredentialStore")
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

	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM credential WHERE identity_id = $1", idn.ID.String())
		_, _ = pool.Exec(bg, "DELETE FROM identity WHERE id = $1", idn.ID.String())
	})
	return pool, NewCredentialStore(pool), idn
}

func TestCredentialStoreCreateAndList(t *testing.T) {
	pool, store, idn := setupCredentialStore(t)
	_ = pool
	ctx := context.Background()

	pw, _ := domain.NewPasswordCredential(idn.ID, []byte("hash"), "bcrypt", "salt")
	totp, _ := domain.NewTOTPCredential(idn.ID, "vault://ref-1")
	wa, _ := domain.NewWebAuthnCredential(idn.ID, []byte("pubkey"))
	rc, _ := domain.NewRecoveryCodeCredential(idn.ID, []byte("codehash"))

	for _, c := range []domain.Credential{pw, totp, wa, rc} {
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("Create %s: %v", c.Type, err)
		}
	}

	got, err := store.ListByIdentity(ctx, idn.ID)
	if err != nil {
		t.Fatalf("ListByIdentity: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("esperava 4 credenciais, veio %d", len(got))
	}
	// A referência de TOTP recompõe; o material de senha e webauthn também.
	for _, c := range got {
		if !c.WellFormed() {
			t.Errorf("credencial %s recomposta não é well-formed", c.Type)
		}
		if c.Type == domain.FactorTOTP && c.SecretRef != "vault://ref-1" {
			t.Errorf("secret_ref recomposto = %q", c.SecretRef)
		}
		if c.Type == domain.FactorPassword && c.Params["algo"] != "bcrypt" {
			t.Errorf("params recompostos = %v", c.Params)
		}
	}
}

func TestCredentialStoreRefusesMalformed(t *testing.T) {
	_, store, idn := setupCredentialStore(t)
	ctx := context.Background()
	// Guarda de aplicação: um TOTP carregando um seed em claro é malformado.
	bad := domain.Credential{ID: idn.ID, IdentityID: idn.ID, Type: domain.FactorTOTP,
		AAL: domain.AAL2, SecretRef: "ref", Verifier: []byte("seed-em-claro")}
	if err := store.Create(ctx, bad); !errors.Is(err, ErrMalformedCredential) {
		t.Errorf("Create malformado: erro = %v, quer ErrMalformedCredential", err)
	}
}

func TestCredentialShapeCheckIsEnforcedByDB(t *testing.T) {
	pool, _, idn := setupCredentialStore(t)
	ctx := context.Background()
	newUUID := func() string {
		var s string
		if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Segundo anteparo (INV-7 no banco): INSERT cru que viola o credential_shape.
	// TOTP com verifier preenchido.
	if _, err := pool.Exec(ctx,
		`INSERT INTO credential (id, identity_id, type, aal, verifier, secret_ref)
		 VALUES ($1, $2, 'totp', 'aal2', $3, 'ref')`, newUUID(), idn.ID.String(), []byte("seed")); err == nil {
		t.Error("TOTP com verifier deveria violar credential_shape")
	}
	// Senha com secret_ref (segredo reversível numa senha) deve violar.
	if _, err := pool.Exec(ctx,
		`INSERT INTO credential (id, identity_id, type, aal, verifier, secret_ref)
		 VALUES ($1, $2, 'password', 'aal1', $3, 'ref')`, newUUID(), idn.ID.String(), []byte("h")); err == nil {
		t.Error("senha com secret_ref deveria violar credential_shape")
	}
	// aal inválido deve violar.
	if _, err := pool.Exec(ctx,
		`INSERT INTO credential (id, identity_id, type, aal, verifier)
		 VALUES ($1, $2, 'password', 'aal9', $3)`, newUUID(), idn.ID.String(), []byte("h")); err == nil {
		t.Error("aal inválido deveria violar o CHECK")
	}
}

func TestCredentialOnePasswordPerIdentity(t *testing.T) {
	_, store, idn := setupCredentialStore(t)
	ctx := context.Background()
	a, _ := domain.NewPasswordCredential(idn.ID, []byte("h1"), "bcrypt", "")
	b, _ := domain.NewPasswordCredential(idn.ID, []byte("h2"), "bcrypt", "")
	if err := store.Create(ctx, a); err != nil {
		t.Fatalf("Create primeira senha: %v", err)
	}
	if err := store.Create(ctx, b); err == nil {
		t.Error("segunda senha para a mesma identidade deveria violar o índice único parcial")
	}
}
