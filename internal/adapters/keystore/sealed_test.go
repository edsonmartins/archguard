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

package keystore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRefusesWithoutMaterial(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "ks"), nil); err != ErrNoSealingMaterial {
		t.Fatalf("sem material deveria falhar com ErrNoSealingMaterial, veio %v", err)
	}
	if _, err := Open(filepath.Join(t.TempDir(), "ks"), []byte{}); err != ErrNoSealingMaterial {
		t.Fatalf("material vazio deveria falhar, veio %v", err)
	}
}

func TestSealUnsealRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ks")
	material := []byte("dev-unseal-material-xyz")

	ks, err := Open(path, material)
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Put("cert-built-in", "PEM-PRIVATE-KEY"); err != nil {
		t.Fatal(err)
	}

	// Reopen with the same material — the sealed key must come back.
	ks2, err := Open(path, material)
	if err != nil {
		t.Fatalf("reabrir com material correto falhou: %v", err)
	}
	if pem, ok := ks2.Get("cert-built-in"); !ok || pem != "PEM-PRIVATE-KEY" {
		t.Fatalf("chave não recuperada: ok=%v pem=%q", ok, pem)
	}
}

func TestWrongMaterialFailsToUnseal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ks")
	ks, _ := Open(path, []byte("right-material"))
	_ = ks.Put("k", "secret")

	if _, err := Open(path, []byte("WRONG-material")); err == nil {
		t.Fatal("material errado deveria falhar o unseal (AES-GCM)")
	}
}

func TestTamperIsDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ks")
	ks, _ := Open(path, []byte("m"))
	_ = ks.Put("k", "secret")

	raw, _ := os.ReadFile(path)
	raw[len(raw)-1] ^= 0xFF // flip a ciphertext byte
	_ = os.WriteFile(path, raw, 0o600)

	if _, err := Open(path, []byte("m")); err == nil {
		t.Fatal("arquivo adulterado deveria falhar a autenticação GCM")
	}
}

func TestKeyMaterialNotInPlaintextOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ks")
	ks, _ := Open(path, []byte("m"))
	_ = ks.Put("k", "SUPER-SECRET-KEY-MATERIAL")

	raw, _ := os.ReadFile(path)
	if len(raw) == 0 {
		t.Fatal("keystore vazio no disco")
	}
	if contains(raw, []byte("SUPER-SECRET-KEY-MATERIAL")) {
		t.Fatal("material da chave apareceu em CLARO no disco (I-4.3)")
	}
}

func contains(hay, needle []byte) bool {
	if len(needle) == 0 || len(hay) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
