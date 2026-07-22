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
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Recovery-code generation parameters. Each code carries codeEntropyBytes bytes
// (80 bits) of randomness — enough that its SHA-256 verifier cannot be brute
// forced, so no slow KDF is needed (unlike a password). defaultRecoveryCodes is
// the size of a freshly-issued set; minRecoveryCodes/maxRecoveryCodes bound a
// caller-requested size.
const (
	codeEntropyBytes     = 10
	defaultRecoveryCodes = 10
	minRecoveryCodes     = 1
	maxRecoveryCodes     = 50
)

// recoveryCodeEncoding renders a code as lowercase base32 without padding — the
// human-readable, unambiguous alphabet (no 0/1/8/9) users transcribe.
var recoveryCodeEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// ErrNoRecoveryCode is returned when a recovery-code input matches none of the
// identity's outstanding codes.
var ErrNoRecoveryCode = errors.New("recovery: código não corresponde a nenhum código válido")

// GenerateRecoveryCodes mints a fresh set of n single-use recovery codes for the
// identity. It returns the PLAINTEXT codes — shown to the user EXACTLY ONCE, then
// discarded, never persisted or logged — paired with the credentials that carry
// only their one-way SHA-256 verifier (INV-7). Issuing a new set is the mass
// invalidation: the caller replaces ALL of the identity's existing recovery
// credentials with this set in a single transaction, so every prior code stops
// working at once. n defaults to defaultRecoveryCodes when zero.
func GenerateRecoveryCodes(identityID uuid.UUID, n int) (plaintext []string, creds []Credential, err error) {
	if n == 0 {
		n = defaultRecoveryCodes
	}
	if n < minRecoveryCodes || n > maxRecoveryCodes {
		return nil, nil, fmt.Errorf("recovery: quantidade de códigos fora da faixa [%d,%d]: %d", minRecoveryCodes, maxRecoveryCodes, n)
	}
	plaintext = make([]string, 0, n)
	creds = make([]Credential, 0, n)
	for i := 0; i < n; i++ {
		code, err := newRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		hash := HashRecoveryCode(code)
		cred, err := NewRecoveryCodeCredential(identityID, hash)
		if err != nil {
			return nil, nil, err
		}
		plaintext = append(plaintext, code)
		creds = append(creds, cred)
	}
	return plaintext, creds, nil
}

// newRecoveryCode draws codeEntropyBytes of cryptographic randomness and formats
// it as a grouped, lowercase base32 string (e.g. "abcd-efgh-ijkl").
func newRecoveryCode() (string, error) {
	buf := make([]byte, codeEntropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("recovery: geração de aleatoriedade falhou: %w", err)
	}
	raw := strings.ToLower(recoveryCodeEncoding.EncodeToString(buf))
	return groupInFours(raw), nil
}

// groupInFours inserts a hyphen every four characters for legibility.
func groupInFours(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// normalizeRecoveryCode makes verification robust to how a user transcribes a
// code: it strips separators/whitespace and lowercases, so "ABCD EFGH" and
// "abcd-efgh" hash to the same verifier.
func normalizeRecoveryCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(code) {
		if r == '-' || r == ' ' || r == '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// HashRecoveryCode is the one-way verifier stored for a recovery code: the
// SHA-256 of its normalized form. High code entropy makes this preimage-safe
// without a slow KDF.
func HashRecoveryCode(code string) []byte {
	sum := sha256.Sum256([]byte(normalizeRecoveryCode(code)))
	return sum[:]
}

// MatchRecoveryCode finds which of the identity's outstanding recovery
// credentials the input satisfies, comparing verifiers in CONSTANT TIME so a
// mismatch leaks no timing signal. It returns the matched credential's id; the
// caller MUST then consume exactly that credential (single-use: delete it so it
// can never be replayed) and audit the use. A no-match is ErrNoRecoveryCode, a
// denial — not a system error. Only recovery_code credentials are considered.
func MatchRecoveryCode(creds []Credential, input string) (uuid.UUID, error) {
	want := HashRecoveryCode(input)
	matched := uuid.Nil
	// Scan ALL credentials without early exit, so the work — and thus the timing —
	// does not depend on the code's position in the set.
	for _, c := range creds {
		if c.Type != FactorRecoveryCode {
			continue
		}
		if subtle.ConstantTimeCompare(c.Verifier, want) == 1 {
			matched = c.ID
		}
	}
	if matched == uuid.Nil {
		return uuid.Nil, ErrNoRecoveryCode
	}
	return matched, nil
}
