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
	"errors"
	"strings"
)

// ErrEmptyEmail is returned when an e-mail normalizes to the empty string — an
// identity without an e-mail (e.g. a service account) simply has a nil
// email_hash; it must never be stored as the hash of "".
var ErrEmptyEmail = errors.New("keycustodian: e-mail vazio após normalização")

// KeyCustodian custodies the deployment's cryptographic material and performs
// operations with it, so keys never leave the custodian (OpenBao in production,
// sealed keystore in dev — ADR-0012/ADR-0014). It is introduced minimally here
// for the e-mail hash; the per-subject encryption operations that back
// crypto-shredding (ADR-0014) are added when the credential/PII migration needs
// them (T-005).
type KeyCustodian interface {
	// HashEmail returns a stable, deployment-keyed HMAC of the e-mail, computed
	// over its normalized form (NormalizeEmail). The same e-mail always yields
	// the same hash within a deployment — that is what lets email_hash back
	// uniqueness and login lookup without ever storing or comparing plaintext.
	// It returns ErrEmptyEmail for an e-mail that normalizes to empty, and never
	// exposes the key.
	HashEmail(email string) ([]byte, error)
}

// NormalizeEmail canonicalizes an e-mail for hashing and uniqueness: it trims
// surrounding whitespace and lowercases it (Unicode-aware). It deliberately does
// NOT strip +tags or local-part dots: aggressive canonicalization could merge
// two distinct people into a single identity, which is unacceptable in a PAM
// (decisão do arquiteto, 2026-07-20). Callers hash the result, never the raw
// input, so the same address always maps to the same hash.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
