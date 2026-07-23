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
	"fmt"

	"github.com/google/uuid"
)

// FactorType is the kind of authentication credential. Credentials belong to the
// global identity, never to a membership (RFC-0002 §2.4): a person authenticates
// once, then chooses a tenant.
type FactorType string

const (
	// FactorPassword: a password. The stored material is a one-way VERIFIER
	// (hash), not a secret — it may live in the database (INV-7).
	FactorPassword FactorType = "password"
	// FactorTOTP: a time-based one-time-password authenticator. The seed is a
	// REVERSIBLE secret and lives in the vault; the credential holds only a
	// SecretRef (INV-7).
	FactorTOTP FactorType = "totp"
	// FactorWebAuthn: a WebAuthn/passkey authenticator. The stored material is a
	// PUBLIC key and credential id — public, database-safe.
	FactorWebAuthn FactorType = "webauthn"
	// FactorRecoveryCode: a single-use recovery code. Stored as a one-way
	// VERIFIER (hash) of the high-entropy code, never in the clear.
	FactorRecoveryCode FactorType = "recovery_code"
)

// Valid reports whether t is a defined factor type.
func (t FactorType) Valid() bool {
	switch t {
	case FactorPassword, FactorTOTP, FactorWebAuthn, FactorRecoveryCode:
		return true
	default:
		return false
	}
}

// AAL is the authenticator assurance level a factor provides (NIST-aligned,
// ADR-0010): AAL1 a single factor (password), AAL2 a strong factor (TOTP, or a
// passkey), AAL3 a phishing-resistant hardware factor. It feeds the L1/L2/L3
// step-up policy of privileged operations.
type AAL string

const (
	AAL1 AAL = "aal1"
	AAL2 AAL = "aal2"
	AAL3 AAL = "aal3"
)

// Valid reports whether a is a defined assurance level.
func (a AAL) Valid() bool {
	return a == AAL1 || a == AAL2 || a == AAL3
}

// aalRank orders the levels: aal1 < aal2 < aal3 (ADR-0010).
var aalRank = map[AAL]int{AAL1: 1, AAL2: 2, AAL3: 3}

// AtLeast reports whether a provides at least the assurance of min — the
// comparison behind step-up decisions (a tenant demanding min rejects a weaker
// proven level). Fail-closed: an undefined level on either side satisfies
// nothing.
func (a AAL) AtLeast(min AAL) bool {
	ra, okA := aalRank[a]
	rm, okMin := aalRank[min]
	return okA && okMin && ra >= rm
}

// Level maps an authenticator assurance level to the assurance CLASS it yields —
// the acr vocabulary of RFC-0006 §3 and the audit context: aal1→L1, aal2→L2,
// aal3→L3 (the inverse of AssuranceLevel.RequiredAAL). It is the SINGLE source of
// the acr representation, so a token's acr, the step-up challenge acr_values and
// the audit trail's acr never disagree. An undefined AAL yields "".
func (a AAL) Level() AssuranceLevel {
	switch a {
	case AAL1:
		return L1
	case AAL2:
		return L2
	case AAL3:
		return L3
	default:
		return ""
	}
}

// DefaultAAL is the conservative assurance level of a freshly-migrated factor.
// WebAuthn is capped at AAL2 here: claiming AAL3 requires user-verification /
// hardware-attestation evidence that a bulk migration does not have — it is
// raised per credential (SetAssurance) when that evidence is established at
// registration (T-002).
func DefaultAAL(t FactorType) AAL {
	switch t {
	case FactorTOTP, FactorWebAuthn, FactorRecoveryCode:
		return AAL2
	default: // password
		return AAL1
	}
}

// MaxAAL is the CEILING assurance a factor type can ever provide (ADR-0010):
// WebAuthn up to AAL3 (phishing-resistant hardware / user-verified), TOTP and
// recovery codes at most AAL2, a password at most AAL1. A credential whose AAL
// exceeds its type's ceiling is not well-formed — which is what makes "TOTP
// cannot satisfy L3" true by construction, not by a runtime check that could be
// forgotten.
func MaxAAL(t FactorType) AAL {
	switch t {
	case FactorWebAuthn:
		return AAL3
	case FactorTOTP, FactorRecoveryCode:
		return AAL2
	default: // password
		return AAL1
	}
}

// Errors from credential construction.
var (
	ErrInvalidCredential = errors.New("credential: dados obrigatórios ausentes")
	ErrInvalidFactorType = errors.New("credential: tipo de fator inválido")
	// ErrAssuranceExceedsCeiling is returned when a factor is assigned an
	// assurance level above what its type can provide (e.g. TOTP claiming AAL3).
	ErrAssuranceExceedsCeiling = errors.New("credential: nível de garantia acima do teto do tipo de fator")
)

// Credential is one authentication factor of an identity (RFC-0002 §2.4). The
// three material fields are mutually exclusive by construction, and this is where
// INV-7 lives structurally:
//
//   - Verifier — a ONE-WAY hash (password, recovery code). Not a secret.
//   - SecretRef — a reference to a REVERSIBLE secret in the vault (TOTP seed).
//     The secret itself is never in this struct nor in the database.
//   - PublicMaterial — PUBLIC key material (WebAuthn). Database-safe.
//
// There is deliberately no field that holds a reversible secret in the clear, so
// no code path can persist one via a Credential.
type Credential struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	Type       FactorType
	AAL        AAL
	Verifier   []byte
	SecretRef  string
	// PublicMaterial holds public factor data (e.g. the serialized WebAuthn
	// public credential). Opaque to the domain.
	PublicMaterial []byte
	// Params is non-secret metadata (password algorithm and salt, TOTP period,
	// WebAuthn sign count…). It must never carry a secret.
	Params map[string]string
	Label  string
}

// NewPasswordCredential builds a password factor from an existing one-way hash
// (the legacy store already hashed it; we never see plaintext) plus the algorithm
// and salt needed to verify later.
func NewPasswordCredential(identityID uuid.UUID, verifier []byte, algo, salt string) (Credential, error) {
	if len(verifier) == 0 {
		return Credential{}, fmt.Errorf("%w: verifier de senha vazio", ErrInvalidCredential)
	}
	c, err := newCredential(identityID, FactorPassword)
	if err != nil {
		return Credential{}, err
	}
	c.Verifier = verifier
	c.Params = map[string]string{"algo": algo, "salt": salt}
	return c, nil
}

// NewTOTPCredential builds a TOTP factor from a vault reference to its seed. It
// takes a reference, never the seed — the seed goes to the SecretStore first
// (INV-7).
func NewTOTPCredential(identityID uuid.UUID, secretRef string) (Credential, error) {
	if secretRef == "" {
		return Credential{}, fmt.Errorf("%w: secretRef de TOTP vazio", ErrInvalidCredential)
	}
	c, err := newCredential(identityID, FactorTOTP)
	if err != nil {
		return Credential{}, err
	}
	c.SecretRef = secretRef
	return c, nil
}

// NewWebAuthnCredential builds a WebAuthn factor from its public credential
// material.
func NewWebAuthnCredential(identityID uuid.UUID, publicMaterial []byte) (Credential, error) {
	if len(publicMaterial) == 0 {
		return Credential{}, fmt.Errorf("%w: material público WebAuthn vazio", ErrInvalidCredential)
	}
	c, err := newCredential(identityID, FactorWebAuthn)
	if err != nil {
		return Credential{}, err
	}
	c.PublicMaterial = publicMaterial
	return c, nil
}

// NewRecoveryCodeCredential builds a recovery-code factor from the one-way hash
// of a high-entropy code (never the code itself).
func NewRecoveryCodeCredential(identityID uuid.UUID, codeHash []byte) (Credential, error) {
	if len(codeHash) == 0 {
		return Credential{}, fmt.Errorf("%w: hash de recovery code vazio", ErrInvalidCredential)
	}
	c, err := newCredential(identityID, FactorRecoveryCode)
	if err != nil {
		return Credential{}, err
	}
	c.Verifier = codeHash
	return c, nil
}

// newCredential mints the id, validates the identity reference and factor type,
// and sets the default assurance level.
func newCredential(identityID uuid.UUID, t FactorType) (Credential, error) {
	if !t.Valid() {
		return Credential{}, fmt.Errorf("%w: %q", ErrInvalidFactorType, t)
	}
	if identityID == uuid.Nil {
		return Credential{}, fmt.Errorf("%w: identidade nula", ErrInvalidCredential)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Credential{}, fmt.Errorf("credential: geração de UUIDv7 falhou: %w", err)
	}
	return Credential{ID: id, IdentityID: identityID, Type: t, AAL: DefaultAAL(t)}, nil
}

// PhishingResistant reports whether the factor resists real-time phishing —
// only WebAuthn does (ADR-0010). It is the gate for L3 operations: a step-up to
// L3 MUST be satisfied by a phishing-resistant factor, so TOTP can never
// satisfy L3 (spec "Fator resistente a phishing para operações críticas").
func (c Credential) PhishingResistant() bool {
	return c.Type == FactorWebAuthn
}

// Strong reports whether the factor counts as a STRONG factor for the
// mandatory-MFA rule (spec "MFA obrigatório"): WebAuthn or TOTP. A password is
// not strong; a recovery code is a break-glass fallback, not a standing factor.
func (c Credential) Strong() bool {
	return c.Type == FactorWebAuthn || c.Type == FactorTOTP
}

// SetAssurance raises (or sets) the credential's assurance level, refusing a
// level above the type's ceiling (ErrAssuranceExceedsCeiling). Registration
// (T-002) calls this to promote a WebAuthn factor to AAL3 once user-verification
// / attestation evidence is established.
func (c *Credential) SetAssurance(a AAL) error {
	if !a.Valid() {
		return fmt.Errorf("%w: %q", ErrAssuranceExceedsCeiling, a)
	}
	if !MaxAAL(c.Type).AtLeast(a) {
		return fmt.Errorf("%w: %s não pode prover %s (teto %s)", ErrAssuranceExceedsCeiling, c.Type, a, MaxAAL(c.Type))
	}
	c.AAL = a
	return nil
}

// WellFormed reports whether the credential carries EXACTLY the material its
// type allows — which is precisely the INV-7 shape — AND an assurance level
// within its type's ceiling. A store MUST refuse to persist a credential that
// is not WellFormed, and tests assert it:
//
//   - password / recovery_code: a one-way Verifier only (no SecretRef, no public).
//   - totp: a vault SecretRef only (no Verifier, no public material) — the seed
//     is never in the clear here or in the database.
//   - webauthn: PublicMaterial only (no Verifier, no SecretRef).
//
// Because a reversible secret can only ever be a SecretRef, a WellFormed
// credential structurally cannot hold a reversible secret in the clear.
func (c Credential) WellFormed() bool {
	if !c.Type.Valid() || !c.AAL.Valid() || c.IdentityID == uuid.Nil {
		return false
	}
	// The assurance level must not exceed the type's ceiling — a TOTP claiming
	// AAL3 (which would let it satisfy an L3 step-up) is not well-formed.
	if !MaxAAL(c.Type).AtLeast(c.AAL) {
		return false
	}
	switch c.Type {
	case FactorPassword, FactorRecoveryCode:
		return len(c.Verifier) > 0 && c.SecretRef == "" && len(c.PublicMaterial) == 0
	case FactorTOTP:
		return c.SecretRef != "" && len(c.Verifier) == 0 && len(c.PublicMaterial) == 0
	case FactorWebAuthn:
		return len(c.PublicMaterial) > 0 && c.SecretRef == "" && len(c.Verifier) == 0
	default:
		return false
	}
}
