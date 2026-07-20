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

// Package deploy models the ArchGuard deployment profiles (ADR-0017). The
// profile is a mandatory, explicit boot-time declaration: dev, pilot or
// production. There is no inferred default — starting without a valid profile is
// a fatal error (I-4.4: a secure default requires a conscious choice).
package deploy

import "fmt"

// Profile is a named deployment configuration (ADR-0017 §1).
type Profile string

const (
	// Dev: core + PostgreSQL, key custody in a local sealed keystore. For
	// development, CI, smoke test and demonstration — NOT supported in
	// production. Marked non-conformant; L3 operations denied.
	Dev Profile = "dev"
	// Pilot: core + PostgreSQL + OpenBao. Pilot/homologation at a customer.
	Pilot Profile = "pilot"
	// Production: core + PostgreSQL + OpenBao (HA) + OpenFGA + OTLP. The only
	// configuration supported commercially.
	Production Profile = "production"
)

// Parse validates a profile string. An empty or unknown value is rejected —
// callers must treat that as a fatal startup error (ADR-0017 §1).
func Parse(s string) (Profile, error) {
	switch Profile(s) {
	case Dev, Pilot, Production:
		return Profile(s), nil
	case "":
		return "", fmt.Errorf("deployment profile is required — set deploymentProfile to one of: dev, pilot, production (ADR-0017)")
	default:
		return "", fmt.Errorf("unknown deployment profile %q — expected one of: dev, pilot, production (ADR-0017)", s)
	}
}

// IsDev reports whether this is the dev profile.
func (p Profile) IsDev() bool { return p == Dev }

// Conformant reports whether the profile is a supported (conformant)
// configuration. Only pilot and production are; dev is explicitly not (§4).
func (p Profile) Conformant() bool { return p == Pilot || p == Production }

// DeniesL3 reports whether L3 (highest-assurance) operations must be denied in
// this profile. The dev profile denies them (ADR-0017 §4): without real key
// custody it does not open a privileged session, approve break-glass or rotate a
// key. L3 operation handlers (packages 004/007) call this as their guard; the
// hook exists now so the rule is in place before those operations are built.
func (p Profile) DeniesL3() bool { return p == Dev }

// KeyCustodian names the key custodian a profile uses, for reporting.
func (p Profile) KeyCustodian() string {
	if p == Dev {
		return "local-sealed-keystore"
	}
	return "openbao"
}

var active Profile

// SetActive records the resolved profile for the running process. Call once at
// startup after Parse succeeds.
func SetActive(p Profile) { active = p }

// Active returns the profile resolved at startup (empty before SetActive).
func Active() Profile { return active }
