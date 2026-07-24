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

// Service-level indicators of the critical paths (pacote 010, T-006 / RFC-0001 §8,
// ADR-0013 design). This is the versioned CATALOG the metrics instrumentation
// registers against (the OTLP export wiring is deploy — archguard-devops). Each
// critical path carries a latency SLI and an error-rate SLI; only two paths have a
// COMMITTED p95 objective in RFC-0001 §8 — the others are measured with the
// objective set at the load test (M2), so they are NOT invented here.

// CriticalPath names a hot path the SLIs cover (design 010 §"SLIs").
type CriticalPath string

const (
	PathOIDCAuthz   CriticalPath = "oidc_authz"   // /authorize decision
	PathTokenIssue  CriticalPath = "token_issue"  // token issuance
	PathTokenRenew  CriticalPath = "token_renew"  // refresh/renewal
	PathMFAValidate CriticalPath = "mfa_validate" // factor validation
	PathAuditWrite  CriticalPath = "audit_write"  // audit trail append
	PathPDPDecision CriticalPath = "pdp_decision" // fine-grained authz decision
	PathVaultCall   CriticalPath = "vault_call"   // key custody / vault calls
)

// SLI is one service-level indicator: the critical path, a human description, and
// the committed p95 latency objective in milliseconds — 0 means "no committed
// objective yet" (measured; the target is set at the M2 load test, RFC-0001 §8).
type SLI struct {
	Path                   CriticalPath
	Description            string
	LatencyObjectiveMillis int
}

// AuthPlaneAvailabilityObjective is the committed monthly availability of the
// authentication plane (RFC-0001 §8): 99.9%.
const AuthPlaneAvailabilityObjective = 0.999

// sliCatalog is THE catalog. Latency objectives are only those RFC-0001 §8
// commits; the rest are 0 (to be set at M2 — never guessed).
var sliCatalog = []SLI{
	{Path: PathOIDCAuthz, Description: "latência e erro da decisão de autorização OIDC (/authorize)"},
	{Path: PathTokenIssue, Description: "latência e erro da emissão de token", LatencyObjectiveMillis: 150},
	{Path: PathTokenRenew, Description: "latência e erro da renovação/refresh de token"},
	{Path: PathMFAValidate, Description: "latência e erro da validação de fator (MFA)"},
	{Path: PathAuditWrite, Description: "latência e erro da gravação na trilha de auditoria"},
	{Path: PathPDPDecision, Description: "latência e erro da decisão do PDP", LatencyObjectiveMillis: 50},
	{Path: PathVaultCall, Description: "latência e erro das chamadas ao cofre"},
}

// SLICatalog returns a copy of the SLI catalog.
func SLICatalog() []SLI {
	return append([]SLI(nil), sliCatalog...)
}

// SLIForPath returns the SLI of a critical path.
func SLIForPath(p CriticalPath) (SLI, bool) {
	for _, s := range sliCatalog {
		if s.Path == p {
			return s, true
		}
	}
	return SLI{}, false
}
