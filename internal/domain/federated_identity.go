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

import "errors"

// FederatedIdentity is the neutral result of a VALIDATED federated login (SAML or
// OIDC) against a curated IdP (pacote 009, RFC-0007 §5.3 / ADR-0015). It carries
// what ArchGuard needs to JIT-provision — the e-mail (dedup key), an external id,
// and the display name — plus the assurance the IdP CLAIMS.
//
// HARD RULE (RFC-0007 §5.3 / design 009): the external IdP's assurance is
// INFORMATIONAL ONLY. A third party's acr NEVER satisfies an ArchGuard L3
// operation — a privileged operation always requires a factor verified by
// ArchGuard itself. Trusting a foreign acr for L3 would surrender control over
// privileged access. AuthorizesL3 makes this non-negotiable in code.
type FederatedIdentity struct {
	Provider    string
	Protocol    FederationProtocol
	ExternalID  string
	Email       string
	DisplayName string
	// IdPACR is the assurance class the external IdP asserted. Recorded for audit,
	// NEVER used to gate an L3 operation.
	IdPACR string
}

// FederationProtocol names how a federated login arrived.
type FederationProtocol string

const (
	FederationSAML FederationProtocol = "saml"
	FederationOIDC FederationProtocol = "oidc"
)

// ErrFederatedEmailRequired is returned when a federated assertion carries no
// e-mail — without it there is no dedup key and JIT cannot proceed.
var ErrFederatedEmailRequired = errors.New("federation: assertion sem e-mail (chave de deduplicação)")

// Validate refuses a federated identity with no e-mail.
func (f FederatedIdentity) Validate() error {
	if f.Email == "" {
		return ErrFederatedEmailRequired
	}
	return nil
}

// AuthorizesL3 ALWAYS returns false: a federated login, however high the IdP's
// claimed acr, never authorizes an ArchGuard L3 operation on its own (RFC-0007
// §5.3). L3 demands a step-up with a factor ArchGuard verified. There is no
// configuration that changes this.
func (f FederatedIdentity) AuthorizesL3() bool { return false }

// ProvenAAL is the assurance a federated login establishes AT ARCHGUARD: AAL1
// (identification). A third-party IdP proves the person is who they say, but it
// does NOT prove an ArchGuard-verified factor — so federation alone never reaches
// AAL2/AAL3. Any L2/L3 operation then requires an ArchGuard step-up (RFC-0007
// §5.3). The IdP's acr is recorded (IdPACR) but never raises this.
func (f FederatedIdentity) ProvenAAL() AAL { return AAL1 }

// PhishingResistant is ALWAYS false for a federated login: ArchGuard verified no
// phishing-resistant factor here. Since an L3 operation REQUIRES phishing
// resistance (AssuranceLevel.RequiresPhishingResistant), this is the mechanical
// guarantee that a third party's acr can never satisfy L3 —
// L3.Satisfies(f.ProvenAAL(), f.PhishingResistant()) is false for every IdP acr.
func (f FederatedIdentity) PhishingResistant() bool { return false }

// ToSyncRecord maps the federated identity to the neutral provisioning record, so
// JIT rides the SAME dedup-by-email path as SCIM and LDAP (never a duplicate
// identity, RFC-0007 §5.3). The record is active (a successful federated login).
func (f FederatedIdentity) ToSyncRecord() DirectorySyncRecord {
	external := f.ExternalID
	if external == "" {
		external = f.Email
	}
	return DirectorySyncRecord{
		ExternalID: external,
		Email:      f.Email,
		Attributes: map[string]string{"email": f.Email, "name": f.DisplayName},
		Active:     true,
	}
}
