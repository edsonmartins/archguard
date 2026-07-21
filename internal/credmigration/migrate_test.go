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

package credmigration

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// fakeSecretStore is an in-memory domain.SecretStore for the mechanism tests.
type fakeSecretStore struct {
	m map[string][]byte
	n int
}

func newFakeStore() *fakeSecretStore { return &fakeSecretStore{m: map[string][]byte{}} }

func (f *fakeSecretStore) Put(_ context.Context, secret []byte) (string, error) {
	f.n++
	ref := "vault://" + strconv.Itoa(f.n)
	cp := make([]byte, len(secret))
	copy(cp, secret)
	f.m[ref] = cp
	return ref, nil
}

func (f *fakeSecretStore) Get(_ context.Context, ref string) ([]byte, error) {
	v, ok := f.m[ref]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return v, nil
}

func (f *fakeSecretStore) Delete(_ context.Context, ref string) error {
	delete(f.m, ref)
	return nil
}

func newID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// assertNoReversibleSecretInClear checks that no credential carries the given
// plaintext secret in any stored field, and that every credential is INV-7
// well-formed.
func assertNoReversibleSecretInClear(t *testing.T, creds []domain.Credential, plaintext []byte) {
	t.Helper()
	for _, c := range creds {
		if !c.WellFormed() {
			t.Errorf("credencial %s malformada (INV-7)", c.Type)
		}
		if len(plaintext) > 0 {
			if bytes.Contains(c.Verifier, plaintext) || bytes.Contains(c.PublicMaterial, plaintext) {
				t.Errorf("segredo em claro vazou no material da credencial %s", c.Type)
			}
			if c.SecretRef == string(plaintext) {
				t.Errorf("secret_ref é o próprio segredo, não uma referência (%s)", c.Type)
			}
		}
	}
}

func TestMigrateFullSet(t *testing.T) {
	idn := newID(t)
	store := newFakeStore()
	seed := "JBSWY3DPEHPK3PXP"
	lc := LegacyCredentials{
		PasswordHash:  "$2a$10$hashedpw",
		PasswordSalt:  "salt",
		PasswordType:  "bcrypt",
		TotpSecret:    seed,
		RecoveryCodes: []string{"code-aaa", "code-bbb", ""},
		WebAuthn:      [][]byte{[]byte("pubkey-1")},
	}
	res, err := Migrate(context.Background(), idn, lc, store)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.ForcePasswordReset {
		t.Error("senha hasheada não deveria forçar reset")
	}
	// password + totp + 2 recovery (o vazio é ignorado) + 1 webauthn = 5.
	if len(res.Credentials) != 5 {
		t.Fatalf("esperava 5 credenciais, veio %d", len(res.Credentials))
	}
	assertNoReversibleSecretInClear(t, res.Credentials, []byte(seed))

	// O seed foi para o cofre, e a referência resolve de volta ao seed.
	var totp *domain.Credential
	for i := range res.Credentials {
		if res.Credentials[i].Type == domain.FactorTOTP {
			totp = &res.Credentials[i]
		}
	}
	if totp == nil {
		t.Fatal("credencial TOTP ausente")
	}
	got, err := store.Get(context.Background(), totp.SecretRef)
	if err != nil {
		t.Fatalf("cofre.Get: %v", err)
	}
	if string(got) != seed {
		t.Errorf("cofre guardou %q, quer o seed %q", got, seed)
	}
}

func TestMigrateRecoveryCodesAreHashed(t *testing.T) {
	idn := newID(t)
	lc := LegacyCredentials{RecoveryCodes: []string{"plain-code-123"}}
	res, err := Migrate(context.Background(), idn, lc, newFakeStore())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(res.Credentials) != 1 {
		t.Fatalf("esperava 1 credencial, veio %d", len(res.Credentials))
	}
	rc := res.Credentials[0]
	if bytes.Equal(rc.Verifier, []byte("plain-code-123")) {
		t.Error("recovery code deveria ser hasheado, não em claro")
	}
	if !bytes.Equal(rc.Verifier, HashRecoveryCode("plain-code-123")) {
		t.Error("verifier não corresponde ao HashRecoveryCode")
	}
}

func TestMigratePlaintextPasswordForcesReset(t *testing.T) {
	idn := newID(t)
	for _, pt := range []string{"", "plain"} {
		lc := LegacyCredentials{PasswordHash: "senha-em-claro", PasswordType: pt}
		res, err := Migrate(context.Background(), idn, lc, newFakeStore())
		if err != nil {
			t.Fatalf("Migrate(%q): %v", pt, err)
		}
		if !res.ForcePasswordReset {
			t.Errorf("tipo %q deveria forçar reset", pt)
		}
		// A senha em claro NÃO pode ser carregada como credencial.
		for _, c := range res.Credentials {
			if c.Type == domain.FactorPassword {
				t.Errorf("tipo %q: senha em claro não deveria virar credencial", pt)
			}
			if bytes.Contains(c.Verifier, []byte("senha-em-claro")) {
				t.Errorf("tipo %q: senha em claro vazou", pt)
			}
		}
	}
}

func TestMigrateEmptyIsClean(t *testing.T) {
	res, err := Migrate(context.Background(), newID(t), LegacyCredentials{}, newFakeStore())
	if err != nil {
		t.Fatalf("Migrate vazio: %v", err)
	}
	if len(res.Credentials) != 0 || res.ForcePasswordReset {
		t.Errorf("migração vazia deveria não produzir nada: %+v", res)
	}
}

func TestMigrateRejectsNilIdentity(t *testing.T) {
	if _, err := Migrate(context.Background(), uuid.Nil, LegacyCredentials{}, newFakeStore()); err == nil {
		t.Error("identidade nula deveria ser rejeitada")
	}
}
