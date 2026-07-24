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

import "context"

// VaultSigner signs data with a key HELD IN THE VAULT — the private key NEVER
// leaves the vault (pacote 010, ADR-0012 / spec "Assinatura de selo: a operação
// ocorre no cofre e a aplicação NOT obtém a chave privada"). It backs both the
// JWKS token-signing key (T-010) and the audit-seal signing key (T-011). Callers
// hold only the public key (for JWKS / verification), never the private material.
//
// A signing failure surfaces as an error; callers on the L3 path treat it as a
// denial (fail-closed) — a token or seal that cannot be signed in the vault is not
// produced.
type VaultSigner interface {
	// Sign returns the signature of data under keyName, computed in the vault.
	Sign(ctx context.Context, keyName string, data []byte) (signature []byte, err error)
	// PublicKey returns the current public key (PEM) of keyName — for publishing
	// JWKS or verifying seals. It is the ONLY key material that ever reaches the
	// application.
	PublicKey(ctx context.Context, keyName string) (publicKeyPEM []byte, err error)
}
