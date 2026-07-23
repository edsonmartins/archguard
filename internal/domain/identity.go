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
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IdentityType distinguishes a person from a machine account. Both are global,
// single per deployment (RFC-0002 §2.1); the distinction drives policy (a human
// identity gets MFA/step-up; a service identity gets scoped tokens).
type IdentityType string

const (
	IdentityHuman   IdentityType = "human"
	IdentityService IdentityType = "service"
)

// Valid reports whether t is a defined identity type. An unset or unknown type
// is rejected at construction — the schema CHECK mirrors this (migration 0002).
func (t IdentityType) Valid() bool {
	return t == IdentityHuman || t == IdentityService
}

// AllowsInteractiveLogin reports whether an identity of this type may authenticate
// through the interactive browser flow. Only a HUMAN identity may (pacote 004
// T-015 / ADR-0008 §4): a service account has no interactive login interface — it
// authenticates by a rotatable credential custodied in the vault — so the login
// flow refuses an interactive attempt by a service account (spec "Login
// interativo de conta de serviço").
func (t IdentityType) AllowsInteractiveLogin() bool {
	return t == IdentityHuman
}

// IdentityStatus is the lifecycle state of the global identity. Deprovisioned is
// terminal: RFC-0002 R5 forbids physical deletion, so an identity is retired by
// crypto-shredding (ADR-0014), never removed. The status field carries that.
type IdentityStatus string

const (
	// IdentityActive: the identity may authenticate and hold memberships.
	IdentityActive IdentityStatus = "active"
	// IdentitySuspended: authentication is blocked; memberships and sessions are
	// terminated by the caller (RFC-0002 R4), but the identity can be reactivated.
	IdentitySuspended IdentityStatus = "suspended"
	// IdentityDeprovisioned: terminal. The subject is retired via crypto-shredding
	// (ADR-0014); there is no path back to active and no physical row deletion.
	IdentityDeprovisioned IdentityStatus = "deprovisioned"
)

// Valid reports whether s is a defined lifecycle status.
func (s IdentityStatus) Valid() bool {
	switch s {
	case IdentityActive, IdentitySuspended, IdentityDeprovisioned:
		return true
	default:
		return false
	}
}

// Errors returned by identity construction and lifecycle transitions.
var (
	// ErrInvalidIdentityType is returned when constructing an identity with a
	// type outside {human, service}.
	ErrInvalidIdentityType = errors.New("identity: tipo inválido")
	// ErrIdentityDeprovisioned is returned when a transition is attempted on a
	// deprovisioned identity — a terminal state (RFC-0002 R5).
	ErrIdentityDeprovisioned = errors.New("identity: identidade deprovisionada é estado terminal")
	// ErrInteractiveLoginForbidden is returned when a service account attempts the
	// interactive browser login flow (T-015): a service account authenticates by a
	// vault-custodied rotatable credential, never interactively.
	ErrInteractiveLoginForbidden = errors.New("identity: conta de serviço não autentica por fluxo interativo")
)

// Identity is the global identity (RFC-0002 §2.1): exactly one per person or
// service account across the whole deployment, independent of how many
// organizations they belong to. Credentials and MFA factors belong to the
// identity, never to a membership.
//
// Personal fields are carried as ciphertext (…Enc []byte) and a keyed hash
// (EmailHash): the domain never sees plaintext PII. Encryption and HMAC are the
// KeyCustodian's job (introduced in a later task); this entity models the shape
// the schema stores and the rules the lifecycle must honor.
type Identity struct {
	// ID is a UUIDv7 — time-ordered, so it clusters well as a primary key while
	// still being globally unique (RFC-0002 §2.1).
	ID uuid.UUID
	// Subject is the opaque, stable identifier exposed as the `sub` claim. It is
	// NEVER the e-mail and never the ID (which would leak creation-time ordering):
	// it is independent random material so an external `sub` reveals nothing.
	Subject string
	Type    IdentityType
	Status  IdentityStatus
	// PrimaryEmailEnc is the e-mail ciphertext (per-subject key, ADR-0014).
	PrimaryEmailEnc []byte
	// EmailHash is an HMAC of the normalized e-mail under the deployment key. It
	// backs uniqueness and login without decrypting anything (RFC-0002 §2.1).
	EmailHash []byte
	// DisplayNameEnc is the display-name ciphertext.
	DisplayNameEnc []byte
	// CreatedAt and UpdatedAt are assigned by the database (DEFAULT now()); they
	// are read back on load. Zero on a freshly constructed, unpersisted identity.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EnsureInteractiveLoginAllowed is the gate the interactive login flow calls
// before authenticating an identity: it refuses a service account
// (ErrInteractiveLoginForbidden), which has no interactive login (T-015).
func (i Identity) EnsureInteractiveLoginAllowed() error {
	if !i.Type.AllowsInteractiveLogin() {
		return fmt.Errorf("%w: identidade %s é do tipo %s", ErrInteractiveLoginForbidden, i.Subject, i.Type)
	}
	return nil
}

// subjectBytes is the length of the opaque subject's random material (128 bits).
const subjectBytes = 16

// NewIdentity mints a fresh global identity: a UUIDv7 id, an opaque random
// subject, and the given type, in the active state. Personal fields are left
// empty for a later layer (which holds the KeyCustodian) to populate. It returns
// ErrInvalidIdentityType for a type outside {human, service}.
func NewIdentity(t IdentityType) (Identity, error) {
	if !t.Valid() {
		return Identity{}, fmt.Errorf("%w: %q", ErrInvalidIdentityType, t)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, fmt.Errorf("identity: geração de UUIDv7 falhou: %w", err)
	}
	subject, err := newSubject()
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		ID:      id,
		Subject: subject,
		Type:    t,
		Status:  IdentityActive,
	}, nil
}

// newSubject produces an opaque, URL-safe, unpadded subject from 128 bits of
// cryptographic randomness — stable once assigned, and free of any embedded
// timestamp or personal data.
func newSubject() (string, error) {
	b := make([]byte, subjectBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("identity: geração de subject falhou: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Suspend blocks authentication for the identity. The caller is responsible for
// the cascade to memberships and sessions (RFC-0002 R4). It is a no-op on an
// already-suspended identity and refuses a deprovisioned one (terminal, R5).
func (i *Identity) Suspend() error {
	if i.Status == IdentityDeprovisioned {
		return ErrIdentityDeprovisioned
	}
	i.Status = IdentitySuspended
	return nil
}

// Reactivate returns a suspended identity to active. It refuses a deprovisioned
// identity — deprovisioning is terminal (RFC-0002 R5).
func (i *Identity) Reactivate() error {
	if i.Status == IdentityDeprovisioned {
		return ErrIdentityDeprovisioned
	}
	i.Status = IdentityActive
	return nil
}

// Deprovision retires the identity permanently. There is no physical deletion
// (RFC-0002 R5): the subject is crypto-shredded (ADR-0014) and the row remains
// so the audit chain and references stay intact. Idempotent.
func (i *Identity) Deprovision() {
	i.Status = IdentityDeprovisioned
}
