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
	"bytes"
	"encoding/json"
	"fmt"

	"golang.org/x/text/unicode/norm"
)

// Canonical produces the deterministic canonical bytes of an audit event's
// CONTENT — the exact input to the hash chain (RFC-0003 §3). Determinism is a
// REQUIREMENT of verifiability: a verifier recomputes these bytes from the
// stored row and must get the identical result, or the proof is worthless.
//
// The canonical form is JSON with:
//   - object keys sorted (encoding/json sorts map[string]any keys);
//   - HTML escaping OFF (so <, >, & are literal and stable);
//   - all string values normalized to Unicode NFC;
//   - occurred_at as an integer microseconds-since-epoch (UTC) — Postgres
//     timestamptz precision, so the value round-trips exactly from storage; no
//     float, no format ambiguity;
//   - the chain fields (seq, prev_hash, hash) ABSENT by construction (they live
//     in SealedEvent, not AuditEvent) — they can never leak into their own hash.
//
// If the format ever changes, AuditSchemaVersion MUST be bumped and a new
// fixed-vector test added; the existing vectors pin this version forever.
func Canonical(e AuditEvent) ([]byte, error) {
	m := map[string]any{
		"schema_version":  e.SchemaVersion,
		"event_id":        e.EventID.String(),
		"organization_id": e.OrganizationID.String(),
		"action":          nfc(string(e.Action)),
		"outcome":         e.SerializedOutcome(),
		"occurred_at_us":  e.OccurredAt.UTC().UnixMicro(),
		"reason":          nfc(e.Reason),
		"actor":           canonicalActor(e.Actor),
		"target": map[string]any{
			"type":  nfc(e.Target.Type),
			"id":    nfc(e.Target.ID),
			"label": nfc(e.Target.Label),
		},
		"context": map[string]any{
			"ip":         nfc(e.Context.IP),
			"user_agent": nfc(e.Context.UserAgent),
			"acr":        nfc(e.Context.AuthContextClass),
			"amr":        nfcSlice(e.Context.AuthMethods),
			"trace_id":   nfc(e.Context.TraceID),
			"pcid":       nfc(e.Context.PrivilegedCorrelationID),
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("audit: canonicalização falhou: %w", err)
	}
	// Encoder.Encode appends a trailing newline; drop it so the canonical bytes
	// are exactly the object.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// canonicalActor renders an actor with nil optionals OMITTED (a nil pointer has
// no stable representation), recursing into the delegation chain (act).
func canonicalActor(a AuditActor) map[string]any {
	m := map[string]any{"identity_subject": nfc(a.IdentitySubject)}
	if a.MembershipID != nil {
		m["membership_id"] = a.MembershipID.String()
	}
	if a.SessionID != nil {
		m["session_id"] = a.SessionID.String()
	}
	if a.Act != nil {
		m["act"] = canonicalActor(*a.Act)
	}
	return m
}

// nfc normalizes a string to Unicode NFC — required so two byte-different but
// canonically-equivalent strings hash the same (RFC-0003 §3).
func nfc(s string) string {
	if s == "" {
		return ""
	}
	return norm.NFC.String(s)
}

// nfcSlice normalizes each element, preserving order (amr is ordered data). A
// nil slice becomes an empty slice so the field is always present and stable.
func nfcSlice(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = nfc(s)
	}
	return out
}
