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

import "github.com/google/uuid"

// DecisionAudit is the context needed to record a PDP decision on the audit trail
// (RFC-0004 §5 / RFC-0003): who asked (Actor — an opaque subject, never a name),
// over what asset (Target), the action being decided, and the session's acr and
// privileged-correlation id. The decision itself (verdict + justification) is
// supplied separately to BuildDecisionAuditInput.
type DecisionAudit struct {
	OrganizationID          uuid.UUID
	Action                  Action
	Actor                   AuditActor
	Target                  AuditTarget
	ACR                     string
	PrivilegedCorrelationID string
}

// BuildDecisionAuditInput turns a PDP decision (and an optional infrastructure
// error) into an AuditEventInput, ATTACHING the decision's justification so the
// trail can answer "por que este acesso foi permitido/negado?" (ADR-0005). It
// preserves the denied-vs-error distinction (INV-6 / T-013):
//
//   - decisionErr != nil ⇒ Failed (serialized "error"): the PDP could not decide;
//     the reason is generic (INV-7 — no store/connection detail leaks to the trail).
//   - dec.Allowed        ⇒ Allowed (serialized "success"): reason is the winning
//     resolution path (e.g. "operator from parent asset_group:g1").
//   - otherwise          ⇒ Denied (serialized "denied"): a computed refusal.
//
// The result still passes through NewAuditEvent, which validates the action and
// stamps the id — this function only assembles the content.
func BuildDecisionAuditInput(rec DecisionAudit, dec Decision, decisionErr error) AuditEventInput {
	// The outcome comes from the single fail-closed gate (T-013), so the audit
	// record can never disagree with the effect the caller enforced.
	_, outcome := DecisionOutcome(dec, decisionErr)
	reason := dec.Justification
	switch outcome {
	case Failed:
		// Deliberately generic: the trail records THAT the PDP failed (and the
		// operation was denied fail-closed), not the raw error, which could carry
		// infrastructure detail (INV-7).
		reason = "PDP indisponível — acesso negado (fail-closed)"
	case Allowed:
		if reason == "" {
			reason = "acesso permitido"
		}
	default:
		if reason == "" {
			reason = "acesso negado"
		}
	}
	return AuditEventInput{
		OrganizationID: rec.OrganizationID,
		Action:         rec.Action,
		Actor:          rec.Actor,
		Outcome:        outcome,
		Target:         rec.Target,
		Reason:         reason,
		Context: AuditContext{
			AuthContextClass:        rec.ACR,
			PrivilegedCorrelationID: rec.PrivilegedCorrelationID,
		},
	}
}
