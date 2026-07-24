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
	"path/filepath"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keystore"
)

func TestCertPrivateKeyPEMResolvesKeystoreReference(t *testing.T) {
	ks, err := keystore.Open(filepath.Join(t.TempDir(), "ks"), []byte("material"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Put("admin/cert-built-in", "REAL-PEM-KEY"); err != nil {
		t.Fatal(err)
	}
	old := devKeystore
	devKeystore = ks
	defer func() { devKeystore = old }()

	// A cert holding a keystore reference resolves to the sealed key.
	ref := &Cert{Owner: "admin", Name: "cert-built-in", PrivateKey: keystoreRefPrefix + "admin/cert-built-in"}
	if got, err := CertPrivateKeyPEM(ref); err != nil || got != "REAL-PEM-KEY" {
		t.Fatalf("referência não resolvida: got=%q err=%v", got, err)
	}

	// A literal key (non-dev / legacy) passes through unchanged.
	lit := &Cert{Owner: "admin", Name: "x", PrivateKey: "LITERAL-PEM"}
	if got, _ := CertPrivateKeyPEM(lit); got != "LITERAL-PEM" {
		t.Fatalf("literal alterado: %q", got)
	}
}

func TestCertPrivateKeyPEMFailsOnMissingSealedKey(t *testing.T) {
	ks, _ := keystore.Open(filepath.Join(t.TempDir(), "ks"), []byte("m"))
	old := devKeystore
	devKeystore = ks
	defer func() { devKeystore = old }()

	ref := &Cert{Owner: "admin", Name: "gone", PrivateKey: keystoreRefPrefix + "admin/gone"}
	if _, err := CertPrivateKeyPEM(ref); err == nil {
		t.Fatal("referência a chave inexistente deveria falhar (não silenciar)")
	}
}

func TestDetectPublicExposureIsLocalSafe(t *testing.T) {
	if !isLocalHost("https://localhost:8000") || !isLocalHost("http://127.0.0.1") {
		t.Error("localhost/127.0.0.1 devem ser considerados locais")
	}
	if isLocalHost("https://id.cliente.com") {
		t.Error("domínio público não é local")
	}
}
