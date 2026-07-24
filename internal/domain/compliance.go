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

// Custody conformance (pacote 010, T-016 / spec "Custódia local em produção → o
// health check reporta instalação não conforme"). Key custody in a LOCAL sealed
// keystore is NEVER production-conformant, regardless of the deployment profile
// label: the private material must live in the vault (ADR-0012). The health check
// surfaces this so a production install running on local custody is unmistakably
// flagged.

// CustodyMode is where cryptographic material is custodied.
type CustodyMode string

const (
	// CustodyLocal: an in-process sealed keystore — dev/test only, NEVER conformant.
	CustodyLocal CustodyMode = "local"
	// CustodyVault: OpenBao — the production custody.
	CustodyVault CustodyMode = "vault"
)

// CustodyConformant reports whether the custody mode is production-conformant.
// Only vault custody is; local custody never is.
func CustodyConformant(mode CustodyMode) bool { return mode == CustodyVault }

// ComplianceReport is the compliance state a health check reports. An installation
// is conformant only when BOTH the deployment profile is a supported one AND the
// custody is in the vault — local custody alone makes it non-conformant.
type ComplianceReport struct {
	Custody           CustodyMode
	ProfileConformant bool
}

// Conformant reports whether the installation is fully conformant.
func (r ComplianceReport) Conformant() bool {
	return r.ProfileConformant && CustodyConformant(r.Custody)
}

// Status is the wire label: "conformant" or "non_conformant".
func (r ComplianceReport) Status() string {
	if r.Conformant() {
		return "conformant"
	}
	return "non_conformant"
}

// Reasons lists why the installation is non-conformant (empty when conformant),
// for the health check to explain the state without leaking anything sensitive.
func (r ComplianceReport) Reasons() []string {
	var reasons []string
	if !CustodyConformant(r.Custody) {
		reasons = append(reasons, "custódia de chaves local (não suportada em produção — use o cofre)")
	}
	if !r.ProfileConformant {
		reasons = append(reasons, "perfil de implantação não suportado em produção")
	}
	return reasons
}
