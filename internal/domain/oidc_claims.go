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
)

// OIDCClaimsVersion is the version of the federation claims contract (RFC-0006
// §3). It travels in every token as ClaimsVersion so a component knows which
// contract it is reading; a semantic change to a v1 claim requires a NEW version,
// never a silent redefinition (RFC-0006 §3 "extensões futuras usam namespace
// próprio; claims da v1 não mudam de semântica sem nova versão").
const OIDCClaimsVersion = "v1"

// ErrInvalidClaims is returned when a claim set is missing a required claim or
// carries a value the contract forbids.
var ErrInvalidClaims = errors.New("oidc: claim set inválido")

// OIDCClaims is the ArchGuard federation token's claim set, version 1 (RFC-0006
// §3). It is deliberately VENDOR-AGNOSTIC — no particularity of the fork leaks
// into a claim — so the cost of ever swapping the IdP stays contained (ADR-0001).
// The struct is a plain, framework-free value; the signing adapter maps it onto a
// JWT. Personal data never appears in the clear: Sub is opaque, and Email is
// present ONLY when an explicit, justified email scope was granted (RFC-0006 §3 /
// I-3.2).
type OIDCClaims struct {
	// Registered claims.
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"` // opaque, stable — never the e-mail
	Audience  string `json:"aud"` // the recipient component (one aud per token)
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`

	// ArchGuard contract claims (RFC-0006 §3). Always for the ACTIVE tenant.
	Organization string    `json:"org"` // active tenant
	MembershipID string    `json:"mid"` // membership in the active tenant
	ACR          string    `json:"acr"` // assurance obtained (L1/L2/L3)
	AMR          []string  `json:"amr"` // methods (pwd, webauthn, otp, federated)
	AuthTime     int64     `json:"auth_time"`
	SessionID    string    `json:"sid"` // for back-channel logout
	Groups       []string  `json:"groups,omitempty"`
	Roles        []string  `json:"roles,omitempty"`
	Act          *ActClaim `json:"act,omitempty"`       // real actor in delegation (pacote 004)
	PCID         string    `json:"pcid,omitempty"`      // privileged-session correlation
	GrantRef     string    `json:"grant_ref,omitempty"` // temporary/break-glass grant reference

	// Email is released ONLY under an explicit email scope, for a component that
	// provably needs it (RFC-0006 §3). It is empty in every other token.
	Email string `json:"email,omitempty"`

	// ClaimsVersion pins the contract version this token was minted under.
	ClaimsVersion string `json:"archguard_claims_version"`
}

// WellFormed reports whether the claim set satisfies the v1 contract: every
// required claim is present and coherent, the acr is a defined level, and the
// contract version is the current one. It does NOT decide policy (whether email
// SHOULD be present is the scoped builder's job, T-006) — it is the structural
// gate the signer runs before minting a token, so a malformed claim set never
// reaches a component.
func (c OIDCClaims) WellFormed() error {
	switch {
	case c.Issuer == "":
		return fmt.Errorf("%w: iss ausente", ErrInvalidClaims)
	case c.Subject == "":
		return fmt.Errorf("%w: sub ausente", ErrInvalidClaims)
	case c.Audience == "":
		return fmt.Errorf("%w: aud ausente", ErrInvalidClaims)
	case c.Organization == "":
		return fmt.Errorf("%w: org (tenant ativo) ausente", ErrInvalidClaims)
	case c.MembershipID == "":
		return fmt.Errorf("%w: mid ausente", ErrInvalidClaims)
	case c.SessionID == "":
		return fmt.Errorf("%w: sid ausente", ErrInvalidClaims)
	}
	if !AssuranceLevel(c.ACR).Valid() {
		return fmt.Errorf("%w: acr %q inválido", ErrInvalidClaims, c.ACR)
	}
	if len(c.AMR) == 0 {
		return fmt.Errorf("%w: amr vazio", ErrInvalidClaims)
	}
	if c.AuthTime <= 0 {
		return fmt.Errorf("%w: auth_time ausente", ErrInvalidClaims)
	}
	if c.IssuedAt <= 0 || c.ExpiresAt <= c.IssuedAt {
		return fmt.Errorf("%w: janela iat/exp inválida", ErrInvalidClaims)
	}
	if c.ClaimsVersion != OIDCClaimsVersion {
		return fmt.Errorf("%w: versão de contrato %q, esperada %q", ErrInvalidClaims, c.ClaimsVersion, OIDCClaimsVersion)
	}
	return nil
}
