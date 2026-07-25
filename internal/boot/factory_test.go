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
	"errors"
	"path/filepath"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/adapters/openbao"
	"github.com/casdoor/casdoor/internal/deploy"
)

func TestFactoryProfileAndPool(t *testing.T) {
	f := NewFactory(deploy.Dev, nil, nil, nil)
	if f.Profile() != deploy.Dev {
		t.Fatalf("Profile() = %v, want %v", f.Profile(), deploy.Dev)
	}
	if f.Pool() != nil {
		t.Fatalf("Pool() should return the pool it was built with (nil here)")
	}
}

func TestCustodyAvailableInDev(t *testing.T) {
	f := NewFactory(deploy.Dev, nil, nil, nil)
	if !f.CustodyAvailable() {
		t.Fatalf("dev profile should have custody available (local/provisional)")
	}
	if err := f.RequireCustody(); err != nil {
		t.Fatalf("RequireCustody in dev should be nil, got %v", err)
	}
}

// TestCustodyFailsClosedInConformant is the INV-6/INV-7 guard: a conformant
// profile must refuse custody until OpenBao is wired, never downgrade to dev
// custody. Covers spec scenario "Adapter de desenvolvimento em perfil conforme".
func TestCustodyFailsClosedInConformant(t *testing.T) {
	for _, p := range []deploy.Profile{deploy.Pilot, deploy.Production} {
		f := NewFactory(p, nil, nil, nil)
		if f.CustodyAvailable() {
			t.Fatalf("profile %v must NOT report custody available (OpenBao not wired)", p)
		}
		if err := f.RequireCustody(); !errors.Is(err, ErrCustodyBackendUnavailable) {
			t.Fatalf("profile %v: RequireCustody want ErrCustodyBackendUnavailable, got %v", p, err)
		}
		// KeyCustodian must also fail closed in conformant.
		if _, err := f.KeyCustodian(); !errors.Is(err, ErrCustodyBackendUnavailable) {
			t.Fatalf("profile %v: KeyCustodian want ErrCustodyBackendUnavailable, got %v", p, err)
		}
	}
}

// TestCustodyConformantWithVault: a conformant profile WITH an OpenBao client has
// custody available and vends the OpenBao-backed custodian + secret store (built
// without calling the vault — the remote call happens on use).
func TestCustodyConformantWithVault(t *testing.T) {
	f := NewFactory(deploy.Production, nil, nil, openbao.New("http://vault:8200", "tok"))
	if !f.CustodyAvailable() {
		t.Fatalf("conformant with a vault should report custody available")
	}
	if cust, err := f.KeyCustodian(); err != nil || cust == nil {
		t.Fatalf("conformant+vault KeyCustodian: cust=%v err=%v", cust, err)
	}
	if ss, err := f.SecretStore(); err != nil || ss == nil {
		t.Fatalf("conformant+vault SecretStore: ss=%v err=%v", ss, err)
	}
}

func TestKeyCustodianDevWithoutKeystoreFailsClosed(t *testing.T) {
	f := NewFactory(deploy.Dev, nil, nil, nil) // no keystore
	if _, err := f.KeyCustodian(); err == nil {
		t.Fatalf("KeyCustodian in dev without a keystore should error, not build a keyless custodian")
	}
}

func TestKeyCustodianDevBuildsFromKeystore(t *testing.T) {
	ks := openTempKeystore(t)
	f := NewFactory(deploy.Dev, nil, ks, nil)

	cust, err := f.KeyCustodian()
	if err != nil {
		t.Fatalf("KeyCustodian: %v", err)
	}
	if cust == nil {
		t.Fatalf("KeyCustodian should return a custodian in dev with a keystore")
	}
	// The custodian must hash e-mails (proves the deployment key seeded correctly).
	if _, err := cust.HashEmail("admin@example.com"); err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
}

// TestDevCustodyKeyStableAcrossReopen is the dedup guarantee: the deployment key
// is generated once and reused, so the same e-mail hashes the same after restart.
func TestDevCustodyKeyStableAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ks.sealed")
	material := []byte("dev-sealing-material")

	ks1, err := keystore.Open(path, material)
	if err != nil {
		t.Fatalf("open ks1: %v", err)
	}
	k1, err := devCustodyKey(ks1)
	if err != nil {
		t.Fatalf("devCustodyKey ks1: %v", err)
	}

	// Reopen the same sealed file (simulating a restart) and read the key back.
	ks2, err := keystore.Open(path, material)
	if err != nil {
		t.Fatalf("open ks2: %v", err)
	}
	k2, err := devCustodyKey(ks2)
	if err != nil {
		t.Fatalf("devCustodyKey ks2: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatalf("custody key must be stable across reopen (dedup depends on it)")
	}
	if len(k1) < 32 {
		t.Fatalf("custody key must be at least 256 bits, got %d bytes", len(k1))
	}
}

func openTempKeystore(t *testing.T) *keystore.SealedKeystore {
	t.Helper()
	ks, err := keystore.Open(filepath.Join(t.TempDir(), "ks.sealed"), []byte("dev-sealing-material"))
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	return ks
}
