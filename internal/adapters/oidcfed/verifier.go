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

// Package oidcfed validates an inbound OIDC ID token from a curated OP and maps it
// to a domain.FederatedIdentity for JIT provisioning (pacote 009, T-011 / RFC-0007
// §5.3). Signature verification (against the OP's JWKS) is delegated to a
// TokenVerifier — backed by the JWT/JWKS infra already in the tree. This package
// adds the fail-closed claim checks (issuer, audience, expiry, nonce, verified
// e-mail) and the neutral mapping, and it NEVER lets the OP's acr authorize L3.
package oidcfed

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// Audience is the OIDC "aud" claim, which is a string OR an array of strings.
type Audience []string

// UnmarshalJSON accepts both the string and array forms.
func (a *Audience) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*a = Audience{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

// Contains reports whether the audience includes clientID.
func (a Audience) Contains(clientID string) bool {
	for _, aud := range a {
		if aud == clientID {
			return true
		}
	}
	return false
}

// IDTokenClaims are the OIDC ID token claims ArchGuard consumes.
type IDTokenClaims struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      Audience `json:"aud"`
	Expiry        int64    `json:"exp"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Nonce         string   `json:"nonce"`
	ACR           string   `json:"acr"`
}

// TokenVerifier verifies an ID token's SIGNATURE against the OP's keys and returns
// its claims. The real implementation reuses the JWT/JWKS infra; a fake satisfies
// it in tests so the claim checks are exercised without live crypto.
type TokenVerifier interface {
	VerifyIDToken(rawIDToken string) (IDTokenClaims, error)
}

// Errors of OIDC ID-token claim validation (all fail-closed).
var (
	ErrIssuer          = errors.New("oidcfed: issuer inesperado")
	ErrAudience        = errors.New("oidcfed: token não endereçado a este cliente")
	ErrExpired         = errors.New("oidcfed: id token expirado")
	ErrNonce           = errors.New("oidcfed: nonce não confere (possível replay)")
	ErrEmailUnverified = errors.New("oidcfed: e-mail não verificado pelo OP — inseguro como chave de dedup")
)

// Verifier validates ID tokens and maps them to federated identities.
type Verifier struct {
	tokens         TokenVerifier
	expectedIssuer string
	clientID       string
	provider       string
	now            func() time.Time
}

// NewVerifier builds the verifier over a token verifier, the expected issuer, the
// client id, and the provider id. now supplies the clock (injected for tests).
func NewVerifier(tokens TokenVerifier, expectedIssuer, clientID, provider string, now func() time.Time) *Verifier {
	return &Verifier{tokens: tokens, expectedIssuer: expectedIssuer, clientID: clientID, provider: provider, now: now}
}

// Verify verifies the raw ID token (signature via the TokenVerifier) and applies
// the fail-closed claim checks, returning the federated identity. expectedNonce,
// when non-empty, MUST match the token's nonce (replay defense).
func (v *Verifier) Verify(rawIDToken, expectedNonce string) (domain.FederatedIdentity, error) {
	claims, err := v.tokens.VerifyIDToken(rawIDToken)
	if err != nil {
		return domain.FederatedIdentity{}, fmt.Errorf("oidcfed: verificação de assinatura falhou: %w", err)
	}
	if claims.Issuer != v.expectedIssuer {
		return domain.FederatedIdentity{}, ErrIssuer
	}
	if !claims.Audience.Contains(v.clientID) {
		return domain.FederatedIdentity{}, ErrAudience
	}
	if v.now().After(time.Unix(claims.Expiry, 0)) {
		return domain.FederatedIdentity{}, ErrExpired
	}
	if expectedNonce != "" && claims.Nonce != expectedNonce {
		return domain.FederatedIdentity{}, ErrNonce
	}
	// A dedup key must be trustworthy: an unverified e-mail from the OP is not
	// accepted (someone could assert any address).
	if claims.Email != "" && !claims.EmailVerified {
		return domain.FederatedIdentity{}, ErrEmailUnverified
	}

	fed := domain.FederatedIdentity{
		Provider:    v.provider,
		Protocol:    domain.FederationOIDC,
		ExternalID:  claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.Name,
		IdPACR:      claims.ACR, // informational only — never gates L3
	}
	if err := fed.Validate(); err != nil {
		return domain.FederatedIdentity{}, err
	}
	return fed, nil
}
