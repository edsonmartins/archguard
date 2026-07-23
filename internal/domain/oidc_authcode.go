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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// authCodeBytes is the entropy of an authorization code secret (160 bits).
const authCodeBytes = 20

// AuthCodeMaxTTL is the maximum lifetime of an authorization code (RFC-0006 §5:
// ≤ 60 s). A code is single-use and short-lived.
const AuthCodeMaxTTL = 60 * time.Second

// Errors of the authorization-code flow.
var (
	ErrInvalidAuthCode = errors.New("authcode: dados obrigatórios ausentes")
	// ErrPKCEVerificationFailed is returned when the presented code_verifier does
	// not match the stored S256 challenge — the exchange is refused.
	ErrPKCEVerificationFailed = errors.New("authcode: verificação PKCE falhou")
	// ErrAuthCodeExpiredOrUsed is returned when a code is past its window or was
	// already redeemed (single-use).
	ErrAuthCodeExpiredOrUsed = errors.New("authcode: código expirado ou já usado")
	// ErrRedirectURIMismatch is returned when the redirect_uri at the token
	// endpoint does not match the one the code was issued for.
	ErrRedirectURIMismatch = errors.New("authcode: redirect_uri não corresponde ao do código")
)

// AuthorizationCode is a single-use authorization code binding an authenticated
// session to a client, its PKCE challenge, the redirect_uri and the granted
// scopes (RFC-0006 §5). Only the HASH of the code secret is held (INV-7 — the
// code goes to the client via the redirect, once).
type AuthorizationCode struct {
	ID             uuid.UUID
	CodeHash       []byte
	ClientID       string
	RedirectURI    string
	CodeChallenge  string // S256 challenge
	SessionID      uuid.UUID
	OrganizationID uuid.UUID
	Scopes         []string
	ExpiresAt      time.Time
	Used           bool
}

// NewAuthorizationCode mints a code for an authenticated session. It returns the
// code SECRET (put in the redirect once) and the record (with the hash). ttl is
// clamped to AuthCodeMaxTTL. codeChallenge must be a non-empty S256 challenge
// (the caller validated the method).
func NewAuthorizationCode(clientID, redirectURI, codeChallenge string, sessionID, organizationID uuid.UUID, scopes []string, ttl time.Duration, now time.Time) (secret string, code AuthorizationCode, err error) {
	if clientID == "" || redirectURI == "" || codeChallenge == "" {
		return "", AuthorizationCode{}, fmt.Errorf("%w: client/redirect/challenge", ErrInvalidAuthCode)
	}
	if sessionID == uuid.Nil || organizationID == uuid.Nil {
		return "", AuthorizationCode{}, fmt.Errorf("%w: sessão/organização", ErrInvalidAuthCode)
	}
	if ttl <= 0 || ttl > AuthCodeMaxTTL {
		ttl = AuthCodeMaxTTL
	}
	buf := make([]byte, authCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", AuthorizationCode{}, fmt.Errorf("authcode: aleatoriedade falhou: %w", err)
	}
	secret = "ac_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf))
	id, err := uuid.NewV7()
	if err != nil {
		return "", AuthorizationCode{}, fmt.Errorf("authcode: UUIDv7 falhou: %w", err)
	}
	return secret, AuthorizationCode{
		ID:             id,
		CodeHash:       HashAuthCode(secret),
		ClientID:       clientID,
		RedirectURI:    redirectURI,
		CodeChallenge:  codeChallenge,
		SessionID:      sessionID,
		OrganizationID: organizationID,
		Scopes:         scopes,
		ExpiresAt:      now.Add(ttl),
	}, nil
}

// HashAuthCode is the one-way verifier of an authorization code secret.
func HashAuthCode(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Redeem validates a code at the token endpoint: it must be unused, unexpired,
// match the redirect_uri, and its stored PKCE challenge must match the presented
// code_verifier (S256). On success the caller marks it used (single-use). Every
// check is fail-closed.
func (c AuthorizationCode) Redeem(redirectURI, codeVerifier string, now time.Time) error {
	if c.Used || !now.Before(c.ExpiresAt) {
		return ErrAuthCodeExpiredOrUsed
	}
	if c.RedirectURI != redirectURI {
		return ErrRedirectURIMismatch
	}
	if !VerifyPKCE(codeVerifier, c.CodeChallenge) {
		return ErrPKCEVerificationFailed
	}
	return nil
}

// VerifyPKCE checks a code_verifier against an S256 code_challenge (RFC 7636):
// base64url(SHA256(verifier)) == challenge, compared in constant time. An empty
// verifier or challenge never matches (fail-closed).
func VerifyPKCE(codeVerifier, codeChallenge string) bool {
	if codeVerifier == "" || codeChallenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(codeChallenge)) == 1
}
