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
	"os"
	"strings"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/xorm-io/core"
)

const keystoreRefPrefix = "keystore:"

var (
	// devKeystore is the sealed local keystore (dev profile only).
	devKeystore *keystore.SealedKeystore
	// vaultSecretStore is the OpenBao KV store that custodies signing keys in a
	// CONFORMANT profile (nil in dev, or when no vault is configured). Set at boot
	// by the composition root via SetVaultSecretStore.
	vaultSecretStore domain.SecretStore
)

// SetVaultSecretStore installs the conformant-profile signing-key custody store
// (OpenBao KV). The composition root calls it at boot when a vault is configured,
// before SealCerts. Passing nil leaves the profile without vault custody.
func SetVaultSecretStore(ss domain.SecretStore) { vaultSecretStore = ss }

// InitDeploymentProfile resolves the mandatory deployment profile (ADR-0017 §1)
// and, in the dev profile, applies the dev guards (§4): a startup warning and a
// refusal to boot under signs of public exposure. Called once at startup, before
// the keystore and the token pipeline.
func InitDeploymentProfile() {
	profile, err := deploy.Parse(conf.GetConfigString("deploymentProfile"))
	if err != nil {
		panic(err) // fatal: no profile is a conscious, secure default (I-4.4)
	}
	deploy.SetActive(profile)

	if profile.IsDev() {
		fmt.Println("WARNING: ArchGuard is running in the 'dev' deployment profile — NOT supported in production. Key custody is a local sealed keystore; L3 operations are denied; this installation is reported as non-conformant (ADR-0017).")
		if reason := detectPublicExposure(); reason != "" {
			panic(fmt.Sprintf("refusing to start: the 'dev' profile shows signs of public exposure (%s). Use the 'production' profile behind OpenBao (ADR-0017 §4).", reason))
		}
	}
}

// detectPublicExposure returns a non-empty reason when the dev profile looks
// like it is being exposed publicly (ADR-0017 §4): a public https issuer/origin.
// It is intentionally conservative — a false negative is a missed guard, but a
// false positive would block legitimate local runs.
func detectPublicExposure() string {
	origin := conf.GetConfigString("origin")
	if origin == "" {
		return ""
	}
	if strings.HasPrefix(origin, "https://") && !isLocalHost(origin) {
		return fmt.Sprintf("public https origin %q", origin)
	}
	return ""
}

func isLocalHost(url string) bool {
	host := url
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.IndexAny(host, ":/"); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal")
}

// InitKeystore opens the sealed local keystore for the dev profile (ADR-0017
// §3). The sealing material comes from the KEYSTORE_UNSEAL_KEY environment
// variable; without it the process refuses to start. Non-dev profiles use
// OpenBao (not wired here) and skip the local keystore.
func InitKeystore() {
	if !deploy.Active().IsDev() {
		return
	}
	material := []byte(os.Getenv("KEYSTORE_UNSEAL_KEY"))
	path := conf.GetConfigString("keystorePath")
	if path == "" {
		path = "conf/keystore.sealed"
	}
	ks, err := keystore.Open(path, material)
	if err != nil {
		panic(fmt.Sprintf("dev keystore: %v (set KEYSTORE_UNSEAL_KEY; ADR-0017 §3)", err))
	}
	devKeystore = ks
}

// DevKeystore returns the open dev sealed keystore, or nil outside the dev
// profile (where it is never opened). The composition root (internal/boot) uses
// it as the material store for the dev key custodian; exposing it here avoids
// boot importing this legacy package.
func DevKeystore() *keystore.SealedKeystore {
	return devKeystore
}

// SealCerts moves plaintext signing keys out of the database into the custody
// store — the sealed keystore in dev, OpenBao KV in a conformant profile — leaving
// only a reference in the database (I-4.3/INV-7). Idempotent: already-referenced
// certs are skipped. No store available (a conformant profile without a vault) is a
// no-op: the key stays plaintext, which /health reports as non-conformant.
func SealCerts() {
	// seal writes keyPEM to the active custody store and returns the reference the
	// DB will hold. The dev keystore keys by the cert id (idempotent Has check); the
	// vault returns an opaque ref.
	var seal func(certID, keyPEM string) (ref string, err error)
	switch {
	case deploy.Active().IsDev() && devKeystore != nil:
		seal = func(certID, keyPEM string) (string, error) {
			if !devKeystore.Has(certID) {
				if err := devKeystore.Put(certID, keyPEM); err != nil {
					return "", err
				}
			}
			return certID, nil
		}
	case !deploy.Active().IsDev() && vaultSecretStore != nil:
		seal = func(_ string, keyPEM string) (string, error) {
			return vaultSecretStore.Put(context.Background(), []byte(keyPEM))
		}
	default:
		return
	}

	certs := []*Cert{}
	if err := ormer.Engine.Find(&certs); err != nil {
		panic(fmt.Sprintf("custódia de certs: leitura falhou: %v", err))
	}
	for _, cert := range certs {
		if cert.PrivateKey == "" || strings.HasPrefix(cert.PrivateKey, keystoreRefPrefix) {
			continue // empty or already sealed
		}
		ref, err := seal(cert.GetId(), cert.PrivateKey)
		if err != nil {
			panic(fmt.Sprintf("custódia de %s falhou: %v", cert.GetId(), err))
		}
		cert.PrivateKey = keystoreRefPrefix + ref
		if _, err := ormer.Engine.ID(core.PK{cert.Owner, cert.Name}).Cols("private_key").Update(cert); err != nil {
			panic(fmt.Sprintf("custódia: atualização de %s falhou: %v", cert.GetId(), err))
		}
	}
}

// CertPrivateKeyPEM returns the PEM signing key for a cert, resolving a keystore
// reference through the active custody store (the sealed keystore in dev, OpenBao
// KV in a conformant profile). This is the single choke point every signing path
// must call instead of reading cert.PrivateKey directly.
func CertPrivateKeyPEM(cert *Cert) (string, error) {
	if cert == nil {
		return "", fmt.Errorf("cert is nil")
	}
	if strings.HasPrefix(cert.PrivateKey, keystoreRefPrefix) {
		ref := strings.TrimPrefix(cert.PrivateKey, keystoreRefPrefix)
		switch {
		case devKeystore != nil:
			pem, ok := devKeystore.Get(ref)
			if !ok {
				return "", fmt.Errorf("sealed key %q not found in keystore", ref)
			}
			return pem, nil
		case vaultSecretStore != nil:
			pem, err := vaultSecretStore.Get(context.Background(), ref)
			if err != nil {
				return "", fmt.Errorf("cofre: resolução da chave %q falhou: %w", ref, err)
			}
			return string(pem), nil
		default:
			return "", fmt.Errorf("cert %s referencia custódia mas nenhuma está aberta (fail-closed)", cert.GetId())
		}
	}
	return cert.PrivateKey, nil
}
