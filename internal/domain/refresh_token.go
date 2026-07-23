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
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// refreshSecretBytes is the entropy of a refresh token secret (160 bits) — high
// enough that its SHA-256 verifier is preimage-safe without a slow KDF.
const refreshSecretBytes = 20

// RefreshTokenStatus is the state of one refresh token within its family.
// active → rotated (superseded by its successor) or revoked (family killed). Only
// an active token may be exchanged; presenting a rotated or revoked one is the
// reuse signal that revokes the whole family (RFC-0006 §5, OAuth 2.0 BCP).
type RefreshTokenStatus string

const (
	RefreshActive  RefreshTokenStatus = "active"
	RefreshRotated RefreshTokenStatus = "rotated"
	RefreshRevoked RefreshTokenStatus = "revoked"
)

// Valid reports whether s is a defined status.
func (s RefreshTokenStatus) Valid() bool {
	return s == RefreshActive || s == RefreshRotated || s == RefreshRevoked
}

// Errors of refresh-token handling.
var (
	ErrInvalidRefreshToken = errors.New("refresh: dados obrigatórios ausentes")
	// ErrRefreshNotActive is returned when rotating a token that is not active.
	ErrRefreshNotActive = errors.New("refresh: apenas token ativo pode ser rotacionado")
	// ErrRefreshReuse is the REUSE signal: a rotated or revoked refresh token was
	// presented again — the caller MUST revoke the entire family (T-008).
	ErrRefreshReuse = errors.New("refresh: reuso de token rotacionado detectado — revogar a família")
)

// NewRefreshSecret mints a refresh token SECRET (returned to the client once) and
// its one-way SHA-256 verifier (stored). The secret is never persisted (INV-7);
// the store keeps only the hash and matches a presented token by hashing it.
func NewRefreshSecret() (secret string, hash []byte, err error) {
	buf := make([]byte, refreshSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("refresh: geração de aleatoriedade falhou: %w", err)
	}
	secret = "rt_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf))
	return secret, HashRefreshToken(secret), nil
}

// HashRefreshToken is the one-way verifier of a refresh token secret.
func HashRefreshToken(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// RefreshToken is one refresh token in a family (RFC-0006 §5). A family is the
// chain of rotations rooted at a session's first refresh token; FamilyID is
// shared by every rotation, so revoking the family on reuse kills them all. Only
// the HASH of the secret is held (INV-7).
type RefreshToken struct {
	ID             uuid.UUID
	FamilyID       uuid.UUID
	SessionID      uuid.UUID
	OrganizationID uuid.UUID
	TokenHash      []byte
	Status         RefreshTokenStatus
	ExpiresAt      time.Time
}

// NewRefreshFamily starts a NEW family with its first (active) token — issued
// when a session first obtains a refresh token. FamilyID and the token id are
// minted fresh.
func NewRefreshFamily(sessionID, organizationID uuid.UUID, tokenHash []byte, expiresAt time.Time) (RefreshToken, error) {
	familyID, err := uuid.NewV7()
	if err != nil {
		return RefreshToken{}, fmt.Errorf("refresh: geração de família falhou: %w", err)
	}
	return newRefreshToken(familyID, sessionID, organizationID, tokenHash, expiresAt)
}

// Rotate supersedes an ACTIVE token: it marks the current one rotated and returns
// the SUCCESSOR — a new active token in the SAME family (RFC-0006 §5 "rotação
// obrigatória"). The presented token must be active (ErrRefreshNotActive
// otherwise). The caller persists both changes atomically and hands the new
// secret to the client.
func (r *RefreshToken) Rotate(newTokenHash []byte, expiresAt time.Time) (RefreshToken, error) {
	if r.Status != RefreshActive {
		return RefreshToken{}, fmt.Errorf("%w: status %s", ErrRefreshNotActive, r.Status)
	}
	r.Status = RefreshRotated
	return newRefreshToken(r.FamilyID, r.SessionID, r.OrganizationID, newTokenHash, expiresAt)
}

// Expired reports whether the token's window has passed at now.
func (r RefreshToken) Expired(now time.Time) bool {
	return !now.Before(r.ExpiresAt)
}

// Usable reports whether the token may be exchanged at now: active AND not
// expired. A rotated/revoked/expired token is not usable — and a rotated/revoked
// one presented for exchange is REUSE (see CheckReuse).
func (r RefreshToken) Usable(now time.Time) bool {
	return r.Status == RefreshActive && !r.Expired(now)
}

// CheckReuse inspects a token that was FOUND by its hash at exchange time and
// reports whether presenting it is a reuse attack: a token whose status is
// rotated or revoked has already been superseded or killed, so presenting it
// again is reuse (ErrRefreshReuse) — the caller revokes the whole family (T-008)
// and raises a high-severity event. An active-but-expired token is simply not
// usable (no error signal beyond normal expiry). An active, unexpired token is
// fine (nil).
func (r RefreshToken) CheckReuse() error {
	if r.Status == RefreshRotated || r.Status == RefreshRevoked {
		return fmt.Errorf("%w: token %s da família %s (status %s)", ErrRefreshReuse, r.ID, r.FamilyID, r.Status)
	}
	return nil
}

func newRefreshToken(familyID, sessionID, organizationID uuid.UUID, tokenHash []byte, expiresAt time.Time) (RefreshToken, error) {
	if sessionID == uuid.Nil || organizationID == uuid.Nil {
		return RefreshToken{}, fmt.Errorf("%w: sessão/organização", ErrInvalidRefreshToken)
	}
	if len(tokenHash) == 0 {
		return RefreshToken{}, fmt.Errorf("%w: hash vazio", ErrInvalidRefreshToken)
	}
	if expiresAt.IsZero() {
		return RefreshToken{}, fmt.Errorf("%w: expiração ausente", ErrInvalidRefreshToken)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return RefreshToken{}, fmt.Errorf("refresh: geração de UUIDv7 falhou: %w", err)
	}
	return RefreshToken{
		ID:             id,
		FamilyID:       familyID,
		SessionID:      sessionID,
		OrganizationID: organizationID,
		TokenHash:      tokenHash,
		Status:         RefreshActive,
		ExpiresAt:      expiresAt,
	}, nil
}
