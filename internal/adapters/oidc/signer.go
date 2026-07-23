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

// Package oidc adapts the ArchGuard federation claims (pacote 006) onto signed
// JWTs and publishes the JWKS. Signing keys are RSA (RS256) — the broadest
// component compatibility (Guacamole, OpenBao, the Java Oracle proxy). In
// production the private keys are custodied in the vault (OpenBao, ADR-0012); the
// signer here holds the material a KeyCustodian handed it and never persists it.
package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

// rsaKeyBits is the modulus size of a signing key. 2048 is the minimum the
// components accept and is FIPS-adequate for token signing.
const rsaKeyBits = 2048

// SigningKey is one RSA signing key with its key id (kid). The kid is published
// in the JWT header and in the JWKS so a component picks the right public key.
type SigningKey struct {
	KID     string
	private *rsa.PrivateKey
}

// GenerateSigningKey mints a fresh RSA signing key with the given kid — used in
// dev/tests and when the custodian rotates. In production the key is generated
// and custodied by the vault; this is the local/dev path.
func GenerateSigningKey(kid string) (SigningKey, error) {
	if kid == "" {
		return SigningKey{}, fmt.Errorf("oidc: kid vazio")
	}
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return SigningKey{}, fmt.Errorf("oidc: geração de chave RSA falhou: %w", err)
	}
	return SigningKey{KID: kid, private: priv}, nil
}

// Signer signs federation tokens with the CURRENT key and publishes a JWKS that
// includes the current key AND the retained previous keys (overlap), so a token
// signed just before a rotation still verifies until it expires (RFC-0006 §7).
type Signer struct {
	current  SigningKey
	previous []SigningKey
}

// NewSigner builds a signer over an initial current key.
func NewSigner(current SigningKey) (*Signer, error) {
	if current.private == nil || current.KID == "" {
		return nil, fmt.Errorf("oidc: chave corrente inválida")
	}
	return &Signer{current: current}, nil
}

// CurrentKID returns the kid tokens are currently signed under.
func (s *Signer) CurrentKID() string { return s.current.KID }

// Sign mints a signed JWT (RS256) from the claims, with the current key's kid in
// the header. It refuses a malformed claim set (WellFormed) so a token that
// violates the contract is never signed.
func (s *Signer) Sign(claims domain.OIDCClaims) (string, error) {
	if err := claims.WellFormed(); err != nil {
		return "", err
	}
	m, err := claimsToMap(claims)
	if err != nil {
		return "", err
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, m)
	tok.Header["kid"] = s.current.KID
	signed, err := tok.SignedString(s.current.private)
	if err != nil {
		return "", fmt.Errorf("oidc: assinatura do token falhou: %w", err)
	}
	return signed, nil
}

// SignLogoutToken mints a signed back-channel logout token (RS256, current kid).
// It refuses a malformed token (WellFormed), so a logout token without the
// back-channel event is never signed.
func (s *Signer) SignLogoutToken(claims domain.LogoutTokenClaims) (string, error) {
	if err := claims.WellFormed(); err != nil {
		return "", err
	}
	b, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("oidc: serialização do logout token falhou: %w", err)
	}
	var m jwt.MapClaims
	if err := json.Unmarshal(b, &m); err != nil {
		return "", fmt.Errorf("oidc: mapeamento do logout token falhou: %w", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, m)
	tok.Header["kid"] = s.current.KID
	// OIDC BCL §2.4: a logout token uses typ "logout+jwt".
	tok.Header["typ"] = "logout+jwt"
	signed, err := tok.SignedString(s.current.private)
	if err != nil {
		return "", fmt.Errorf("oidc: assinatura do logout token falhou: %w", err)
	}
	return signed, nil
}

// Rotate installs a new current key, retaining the outgoing one in the published
// set (overlap). The caller retires a previous key only AFTER the longest token
// TTL has elapsed, so no in-flight token is ever orphaned. keepPrevious bounds
// how many old keys stay published.
func (s *Signer) Rotate(newKey SigningKey, keepPrevious int) error {
	if newKey.private == nil || newKey.KID == "" {
		return fmt.Errorf("oidc: nova chave inválida")
	}
	s.previous = append([]SigningKey{s.current}, s.previous...)
	if keepPrevious >= 0 && len(s.previous) > keepPrevious {
		s.previous = s.previous[:keepPrevious]
	}
	s.current = newKey
	return nil
}

// JWKS returns the public JSON Web Key Set: the current key and every retained
// previous key, each with its kid. A component honoring the set can validate both
// tokens minted under the new key and those still circulating under the old one.
func (s *Signer) JWKS() ([]byte, error) {
	set := jose.JSONWebKeySet{}
	for _, k := range append([]SigningKey{s.current}, s.previous...) {
		set.Keys = append(set.Keys, jose.JSONWebKey{
			Key:       &k.private.PublicKey,
			KeyID:     k.KID,
			Algorithm: "RS256",
			Use:       "sig",
		})
	}
	b, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("oidc: serialização do JWKS falhou: %w", err)
	}
	return b, nil
}

// Verify parses a token, checks its RS256 signature against the key named by its
// `kid` (current or a retained previous key), and returns the claims. It does NOT
// validate expiry — liveness is decided by introspection, which composes this
// with a session-liveness check (T-010). A token whose kid is unknown or whose
// signature is invalid is rejected.
func (s *Signer) Verify(tokenStr string) (domain.OIDCClaims, error) {
	m := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, m, func(tok *jwt.Token) (interface{}, error) {
		kid, _ := tok.Header["kid"].(string)
		key, ok := s.publicKeyByKID(kid)
		if !ok {
			return nil, fmt.Errorf("oidc: kid %q desconhecido", kid)
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithoutClaimsValidation())
	if err != nil {
		return domain.OIDCClaims{}, fmt.Errorf("oidc: verificação do token falhou: %w", err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return domain.OIDCClaims{}, fmt.Errorf("oidc: remontagem dos claims falhou: %w", err)
	}
	var claims domain.OIDCClaims
	if err := json.Unmarshal(b, &claims); err != nil {
		return domain.OIDCClaims{}, fmt.Errorf("oidc: decodificação dos claims falhou: %w", err)
	}
	return claims, nil
}

// publicKeyByKID returns the RSA public key for a kid among the current and
// retained previous keys.
func (s *Signer) publicKeyByKID(kid string) (*rsa.PublicKey, bool) {
	for _, k := range append([]SigningKey{s.current}, s.previous...) {
		if k.KID == kid {
			return &k.private.PublicKey, true
		}
	}
	return nil, false
}

// claimsToMap converts the domain claims to the map form the JWT library signs,
// preserving the exact claim names (json tags) of the contract.
func claimsToMap(claims domain.OIDCClaims) (jwt.MapClaims, error) {
	b, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("oidc: serialização dos claims falhou: %w", err)
	}
	var m jwt.MapClaims
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("oidc: mapeamento dos claims falhou: %w", err)
	}
	return m, nil
}
