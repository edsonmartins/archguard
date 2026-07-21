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

// Package credmigration maps a legacy user's credentials and MFA factors onto
// identity-scoped domain credentials (RFC-0002 §2.4, pacote 002 T-005). It is the
// MECHANISM: it moves the reversible TOTP seed into the vault, hashes recovery
// codes, and carries password/WebAuthn material — but it neither persists the
// credentials nor scrubs the legacy secrets. Bulk execution (with backup and a
// rehearsal on a production copy) is T-019.
//
// It takes primitive legacy inputs rather than the XORM `object.User`, so it does
// not reach for the framework/ORM and stays unit-testable.
package credmigration

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// LegacyCredentials is the credential/MFA material extracted from a legacy user.
// TotpSecret and RecoveryCodes arrive in the CLEAR (the legacy schema stored them
// so, an INV-7 defect this migration exists to correct); Migrate removes them
// from the clear.
type LegacyCredentials struct {
	PasswordHash  string   // already hashed by the legacy store (unless plaintext scheme)
	PasswordSalt  string   // salt used by the legacy hash
	PasswordType  string   // legacy hashing scheme; "" or "plain" means PLAINTEXT
	TotpSecret    string   // CLEAR TOTP seed — moved to the vault
	RecoveryCodes []string // CLEAR recovery codes — hashed
	WebAuthn      [][]byte // serialized PUBLIC WebAuthn credentials
}

// Result is the outcome of a per-user credential migration.
type Result struct {
	// Credentials are the identity-scoped, INV-7-well-formed credentials to
	// persist.
	Credentials []domain.Credential
	// ForcePasswordReset is true when the legacy password could not be migrated
	// as a usable verifier (it was stored in plaintext): no password credential
	// is produced and the identity must set a new password on next login. A
	// plaintext secret is NEVER carried forward.
	ForcePasswordReset bool
}

// Migrate builds the identity's credentials from its legacy material. The TOTP
// seed is written to the vault (SecretStore) and replaced by a reference; the
// vault write happens here, OUTSIDE any database transaction (RFC-0004 §4).
func Migrate(ctx context.Context, identityID uuid.UUID, lc LegacyCredentials, secrets domain.SecretStore) (Result, error) {
	if identityID == uuid.Nil {
		return Result{}, fmt.Errorf("credmigration: identidade nula")
	}
	var res Result

	// Password: carry the existing hash, unless the legacy scheme is plaintext —
	// in which case we refuse to carry it and force a reset (INV-1/INV-7).
	if lc.PasswordHash != "" {
		if isPlaintextScheme(lc.PasswordType) {
			res.ForcePasswordReset = true
		} else {
			c, err := domain.NewPasswordCredential(identityID, []byte(lc.PasswordHash), lc.PasswordType, lc.PasswordSalt)
			if err != nil {
				return Result{}, fmt.Errorf("credmigration: senha: %w", err)
			}
			res.Credentials = append(res.Credentials, c)
		}
	}

	// TOTP: move the reversible seed to the vault; keep only the reference.
	if lc.TotpSecret != "" {
		ref, err := secrets.Put(ctx, []byte(lc.TotpSecret))
		if err != nil {
			return Result{}, fmt.Errorf("credmigration: selagem do seed TOTP no cofre falhou: %w", err)
		}
		c, err := domain.NewTOTPCredential(identityID, ref)
		if err != nil {
			return Result{}, fmt.Errorf("credmigration: TOTP: %w", err)
		}
		res.Credentials = append(res.Credentials, c)
	}

	// Recovery codes: store one-way hashes of each high-entropy code.
	for _, code := range lc.RecoveryCodes {
		if code == "" {
			continue
		}
		c, err := domain.NewRecoveryCodeCredential(identityID, HashRecoveryCode(code))
		if err != nil {
			return Result{}, fmt.Errorf("credmigration: recovery code: %w", err)
		}
		res.Credentials = append(res.Credentials, c)
	}

	// WebAuthn: public material, carried as-is.
	for _, pub := range lc.WebAuthn {
		if len(pub) == 0 {
			continue
		}
		c, err := domain.NewWebAuthnCredential(identityID, pub)
		if err != nil {
			return Result{}, fmt.Errorf("credmigration: WebAuthn: %w", err)
		}
		res.Credentials = append(res.Credentials, c)
	}

	return res, nil
}

// HashRecoveryCode returns the one-way hash stored for a recovery code. Recovery
// codes are high-entropy random values, so a single SHA-256 is an adequate,
// deterministic verifier (matching how opaque tokens are stored) — no per-code
// salt is needed.
func HashRecoveryCode(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}

// isPlaintextScheme reports whether a legacy password type means the stored
// value is the password itself rather than a hash.
func isPlaintextScheme(passwordType string) bool {
	return passwordType == "" || passwordType == "plain"
}
