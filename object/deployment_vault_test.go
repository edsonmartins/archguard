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

package object

import (
	"context"
	"fmt"
	"testing"
)

// fakeSecretStore is an in-memory domain.SecretStore for the resolution test.
type fakeSecretStore struct{ secrets map[string][]byte }

func (f *fakeSecretStore) Put(_ context.Context, secret []byte) (string, error) {
	ref := fmt.Sprintf("ref-%d", len(f.secrets)+1)
	f.secrets[ref] = secret
	return ref, nil
}

func (f *fakeSecretStore) Get(_ context.Context, ref string) ([]byte, error) {
	s, ok := f.secrets[ref]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return s, nil
}

func (f *fakeSecretStore) Delete(_ context.Context, ref string) error {
	delete(f.secrets, ref)
	return nil
}

// TestCertPrivateKeyPEMResolvesFromVault: in a conformant profile (no dev keystore,
// a vault store set), CertPrivateKeyPEM resolves a keystore reference from the vault
// — the key is never read from the DB column.
func TestCertPrivateKeyPEMResolvesFromVault(t *testing.T) {
	prevKS, prevVault := devKeystore, vaultSecretStore
	t.Cleanup(func() { devKeystore, vaultSecretStore = prevKS, prevVault })

	devKeystore = nil
	vaultSecretStore = &fakeSecretStore{secrets: map[string][]byte{"ref-1": []byte("PRIVATE-KEY-PEM")}}

	pem, err := CertPrivateKeyPEM(&Cert{PrivateKey: keystoreRefPrefix + "ref-1"})
	if err != nil {
		t.Fatalf("CertPrivateKeyPEM: %v", err)
	}
	if pem != "PRIVATE-KEY-PEM" {
		t.Fatalf("pem = %q, want the vaulted key", pem)
	}

	// A missing vault reference fails closed (never an empty key silently).
	if _, err := CertPrivateKeyPEM(&Cert{PrivateKey: keystoreRefPrefix + "absent"}); err == nil {
		t.Fatalf("missing vault key must error (fail-closed)")
	}
}

// TestCertPrivateKeyPEMFailsClosedWithoutStore: a keystore reference with NO custody
// store open is a fail-closed error — never the (absent) plaintext.
func TestCertPrivateKeyPEMFailsClosedWithoutStore(t *testing.T) {
	prevKS, prevVault := devKeystore, vaultSecretStore
	t.Cleanup(func() { devKeystore, vaultSecretStore = prevKS, prevVault })
	devKeystore, vaultSecretStore = nil, nil

	if _, err := CertPrivateKeyPEM(&Cert{PrivateKey: keystoreRefPrefix + "x"}); err == nil {
		t.Fatalf("keystore ref with no store must error")
	}
}
