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
	"fmt"
	"os"
	"strings"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/xorm-io/core"
)

const keystoreRefPrefix = "keystore:"

var devKeystore *keystore.SealedKeystore

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

// SealCerts moves signing private keys out of the database and into the sealed
// keystore for the dev profile (I-4.3: the database holds a reference, not the
// key). Idempotent: certs already referenced are skipped. No-op outside dev.
func SealCerts() {
	if !deploy.Active().IsDev() || devKeystore == nil {
		return
	}
	certs := []*Cert{}
	if err := ormer.Engine.Find(&certs); err != nil {
		panic(fmt.Sprintf("dev keystore: leitura de certs falhou: %v", err))
	}
	for _, cert := range certs {
		if cert.PrivateKey == "" || strings.HasPrefix(cert.PrivateKey, keystoreRefPrefix) {
			continue // empty or already sealed
		}
		id := cert.GetId()
		if !devKeystore.Has(id) {
			if err := devKeystore.Put(id, cert.PrivateKey); err != nil {
				panic(fmt.Sprintf("dev keystore: selagem de %s falhou: %v", id, err))
			}
		}
		// Replace the plaintext key in the database with a reference (I-4.3).
		cert.PrivateKey = keystoreRefPrefix + id
		if _, err := ormer.Engine.ID(core.PK{cert.Owner, cert.Name}).Cols("private_key").Update(cert); err != nil {
			panic(fmt.Sprintf("dev keystore: atualização de %s falhou: %v", id, err))
		}
	}
}

// CertPrivateKeyPEM returns the PEM signing key for a cert, resolving a
// keystore reference through the sealed keystore. This is the single choke point
// every signing path must call instead of reading cert.PrivateKey directly.
func CertPrivateKeyPEM(cert *Cert) (string, error) {
	if cert == nil {
		return "", fmt.Errorf("cert is nil")
	}
	if strings.HasPrefix(cert.PrivateKey, keystoreRefPrefix) {
		if devKeystore == nil {
			return "", fmt.Errorf("cert %s references the sealed keystore but it is not open", cert.GetId())
		}
		ref := strings.TrimPrefix(cert.PrivateKey, keystoreRefPrefix)
		pem, ok := devKeystore.Get(ref)
		if !ok {
			return "", fmt.Errorf("sealed key %q not found in keystore", ref)
		}
		return pem, nil
	}
	return cert.PrivateKey, nil
}
