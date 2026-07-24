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

// DecisionOutcome collapses a PDP Check result into the FINAL authorization effect
// and the audit outcome. It is the single choke point every caller of the PDP
// routes through, and it is the code-level statement of INV-6 (RFC-0004 §6):
//
//   - err != nil  ⇒ (granted=false, Failed):  the PDP could not decide → DENY,
//     audited as `error`. Error DOMINATES: even a non-empty dec is ignored, so a
//     stale or half-built Decision accompanying an error can never grant access.
//   - dec.Allowed ⇒ (granted=true,  Allowed): the only path to access.
//   - otherwise   ⇒ (granted=false, Denied):  a computed refusal, audited as
//     `denied`.
//
// There is NO parameter, flag, or configuration by which err yields granted — the
// "modo permissivo" the spec forbids ("Ausência de configuração permissiva") does
// not exist because it cannot be expressed here.
func DecisionOutcome(dec Decision, err error) (granted bool, outcome Outcome) {
	if err != nil {
		return false, Failed
	}
	if dec.Allowed {
		return true, Allowed
	}
	return false, Denied
}
