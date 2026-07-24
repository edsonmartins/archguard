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

// Package samlfed validates inbound SAML 2.0 assertions from a curated IdP and maps
// them to a domain.FederatedIdentity for JIT provisioning (pacote 009, T-010 /
// RFC-0007 §5.3). Signature validation is delegated to gosaml2 (goxmldsig) — both
// already in the tree, Apache-2.0. This package adds the fail-closed condition
// checks and the neutral mapping, and it NEVER lets the IdP's acr authorize L3
// (domain.FederatedIdentity.AuthorizesL3 is always false).
package samlfed

import (
	"errors"
	"fmt"
	"strings"

	"github.com/casdoor/casdoor/internal/domain"
	saml2 "github.com/russellhaering/gosaml2"
)

// AssertionValidator validates a SAML Response's signature and returns the
// assertion info — *saml2.SAMLServiceProvider satisfies it via
// RetrieveAssertionInfo. Abstracted so the mapping is testable without a signed
// fixture.
type AssertionValidator interface {
	RetrieveAssertionInfo(encodedResponse string) (*saml2.AssertionInfo, error)
}

// ErrAssertionConditions is returned when an assertion's conditions fail: it is
// outside its validity window or not addressed to this SP's audience. Fail-closed
// — a condition failure is a rejection, never a warning we proceed past.
var ErrAssertionConditions = errors.New("samlfed: condições da assertion não satisfeitas (janela/audiência)")

// Validator maps validated SAML assertions to federated identities.
type Validator struct {
	sp        AssertionValidator
	provider  string
	emailAttr string
	nameAttr  string
	acrAttr   string
}

// NewValidator builds the validator over a SAML SP, the provider id, and the IdP
// attribute names that carry the e-mail, the display name and (optionally) the acr.
func NewValidator(sp AssertionValidator, provider, emailAttr, nameAttr, acrAttr string) *Validator {
	return &Validator{sp: sp, provider: provider, emailAttr: emailAttr, nameAttr: nameAttr, acrAttr: acrAttr}
}

// ValidateResponse validates the encoded SAML Response (signature via gosaml2) and
// returns the federated identity. A bad signature surfaces as an error from the SP;
// a condition failure is ErrAssertionConditions; a missing e-mail is
// domain.ErrFederatedEmailRequired.
func (v *Validator) ValidateResponse(encodedResponse string) (domain.FederatedIdentity, error) {
	info, err := v.sp.RetrieveAssertionInfo(encodedResponse)
	if err != nil {
		return domain.FederatedIdentity{}, fmt.Errorf("samlfed: validação da assertion falhou: %w", err)
	}
	return v.fromAssertionInfo(info)
}

// fromAssertionInfo applies the fail-closed condition checks and maps the assertion
// to a federated identity. It is the security-relevant post-processing, separated
// so it is tested directly (the signature check is gosaml2's).
func (v *Validator) fromAssertionInfo(info *saml2.AssertionInfo) (domain.FederatedIdentity, error) {
	if info == nil {
		return domain.FederatedIdentity{}, errors.New("samlfed: assertion vazia")
	}
	// Fail-closed on conditions: an out-of-window or wrong-audience assertion is
	// rejected, never accepted with a warning.
	if info.WarningInfo != nil && (info.WarningInfo.InvalidTime || info.WarningInfo.NotInAudience) {
		return domain.FederatedIdentity{}, ErrAssertionConditions
	}

	email := info.Values.Get(v.emailAttr)
	if email == "" && looksLikeEmail(info.NameID) {
		email = info.NameID
	}
	fed := domain.FederatedIdentity{
		Provider:    v.provider,
		Protocol:    domain.FederationSAML,
		ExternalID:  info.NameID,
		Email:       email,
		DisplayName: info.Values.Get(v.nameAttr),
		IdPACR:      info.Values.Get(v.acrAttr), // informational only — never gates L3
	}
	if err := fed.Validate(); err != nil {
		return domain.FederatedIdentity{}, err
	}
	return fed, nil
}

// looksLikeEmail is a cheap heuristic to accept an emailAddress-format NameID as
// the e-mail when the IdP sends no explicit e-mail attribute.
func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && !strings.ContainsAny(s, " ")
}
