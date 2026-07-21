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

package domain

import (
	"context"
	"errors"
)

// ErrSecretNotFound is returned by SecretStore.Get when the reference resolves
// to nothing.
var ErrSecretNotFound = errors.New("secretstore: segredo não encontrado")

// SecretStore custodies reversible secrets (a TOTP seed, a client secret) OUTSIDE
// the database and returns an opaque reference to store in its place. This is the
// mechanism behind INV-7 / I-4.3: the reversible secret is written to the vault
// (OpenBao in production, the sealed keystore in dev), and the database — and any
// log or trace — holds only the reference, never the secret.
//
// A one-way verifier (a password or recovery-code HASH) is NOT a reversible
// secret and does not belong here; it is stored directly as a credential
// verifier. SecretStore is only for material that must be recovered in the clear
// to be used.
type SecretStore interface {
	// Put writes secret to the vault and returns an opaque reference. Callers
	// store the reference, never the secret. Being a vault write, it may be a
	// remote call — it MUST NOT run inside a database transaction (RFC-0004 §4).
	Put(ctx context.Context, secret []byte) (ref string, err error)
	// Get resolves a reference back to the secret, or ErrSecretNotFound. The
	// secret is returned for use in memory; it is never persisted by the caller.
	Get(ctx context.Context, ref string) (secret []byte, err error)
}
