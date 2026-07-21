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

// Package secretstore holds implementations of the domain.SecretStore port.
//
// Provisional is NOT SUPPORTED IN PRODUCTION: it keeps reversible secrets in the
// dev sealed keystore (AES-256-GCM at rest, off the database — ADR-0017 §3), with
// no rotation, leasing or access policy. It exists so the credential/MFA model
// can be built and tested before OpenBao lands (ADR-0012). The `production`
// profile MUST wire the port to OpenBao; the dev profile may use this.
package secretstore

import (
	"context"
	"encoding/base64"

	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// refPrefix namespaces secret references inside the shared sealed keystore, so a
// secret ref never collides with a signing-key name.
const refPrefix = "secret/"

// Provisional stores reversible secrets in a SealedKeystore. It satisfies
// domain.SecretStore.
type Provisional struct {
	ks *keystore.SealedKeystore
}

// NewProvisional builds a Provisional secret store over an already-opened sealed
// keystore.
func NewProvisional(ks *keystore.SealedKeystore) *Provisional {
	return &Provisional{ks: ks}
}

// Put seals the secret under a fresh opaque reference and returns the reference.
// The secret is base64-encoded because the keystore stores strings. It never
// returns or logs the secret.
func (p *Provisional) Put(_ context.Context, secret []byte) (string, error) {
	ref := refPrefix + uuid.NewString()
	if err := p.ks.Put(ref, base64.StdEncoding.EncodeToString(secret)); err != nil {
		return "", err
	}
	return ref, nil
}

// Get resolves a reference to its secret, or domain.ErrSecretNotFound.
func (p *Provisional) Get(_ context.Context, ref string) ([]byte, error) {
	enc, ok := p.ks.Get(ref)
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return base64.StdEncoding.DecodeString(enc)
}

// Delete removes a sealed secret. Idempotent (an absent reference is a no-op),
// so it is safe as compensation after a failed transaction.
func (p *Provisional) Delete(_ context.Context, ref string) error {
	return p.ks.Delete(ref)
}

// compile-time check that Provisional satisfies the port.
var _ domain.SecretStore = (*Provisional)(nil)
