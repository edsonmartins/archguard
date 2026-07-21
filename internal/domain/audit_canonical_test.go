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
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fixedEvent builds a fully-specified event with FIXED ids and time, for the
// golden-vector tests. Nothing here depends on the wall clock or randomness, so
// the canonical bytes are reproducible forever.
func fixedEvent(t *testing.T) AuditEvent {
	t.Helper()
	mid := uuid.MustParse("018f9a10-0000-7000-8000-000000000abc")
	sid := uuid.MustParse("018f9a10-0000-7000-8000-000000000def")
	return AuditEvent{
		SchemaVersion:  AuditSchemaVersion,
		EventID:        uuid.MustParse("018f9a00-0000-7000-8000-000000000001"),
		OrganizationID: uuid.MustParse("018f9a00-0000-7000-8000-0000000000ff"),
		Action:         ActionAuthLogin,
		Actor:          AuditActor{IdentitySubject: "sub-opaque", MembershipID: &mid, SessionID: &sid},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "identity", ID: "sub-opaque", Label: "login"},
		Reason:         "fator forte",
		Context: AuditContext{
			IP: "203.0.113.7", UserAgent: "UA/1.0", AuthContextClass: "L1",
			AuthMethods: []string{"pwd", "webauthn"}, TraceID: "trace-1", PrivilegedCorrelationID: "",
		},
		OccurredAt: time.Unix(1_700_000_000, 123_456_000).UTC(),
	}
}

// FIXED VECTOR: the exact canonical bytes and their SHA-256 for fixedEvent.
// Pinning these makes any silent change to the canonicalization break the build
// — which is the whole point (a drift here invalidates historical verification).
const (
	goldenCanonical = `{"action":"auth.login","actor":{"identity_subject":"sub-opaque","membership_id":"018f9a10-0000-7000-8000-000000000abc","session_id":"018f9a10-0000-7000-8000-000000000def"},"context":{"acr":"L1","amr":["pwd","webauthn"],"ip":"203.0.113.7","pcid":"","trace_id":"trace-1","user_agent":"UA/1.0"},"event_id":"018f9a00-0000-7000-8000-000000000001","occurred_at_us":1700000000123456,"organization_id":"018f9a00-0000-7000-8000-0000000000ff","outcome":"success","reason":"fator forte","schema_version":1,"target":{"id":"sub-opaque","label":"login","type":"identity"}}`
	goldenHashHex   = "6a3cdd51332a4375731ad3f4c64dab582536419a577af172132ebb65384f74fe"
)

func TestCanonicalGoldenVector(t *testing.T) {
	got, err := Canonical(fixedEvent(t))
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if string(got) != goldenCanonical {
		t.Fatalf("canonical bytes divergiram do vetor fixo:\n got: %s\nquero: %s", got, goldenCanonical)
	}
	// The chain hashes H(prev||canonical); here we pin H(canonical) alone.
	sum := sha256.Sum256(got)
	if h := hex.EncodeToString(sum[:]); h != goldenHashHex {
		t.Fatalf("hash do canônico = %s, quero %s", h, goldenHashHex)
	}
}

func TestCanonicalDeterministic(t *testing.T) {
	e := fixedEvent(t)
	a, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	b, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("canonicalização não determinística")
	}
}

// NFC: uma string decomposta (e + combining acute) canonicaliza igual à
// composta (é), senão dois eventos equivalentes hasheariam diferente.
func TestCanonicalNFC(t *testing.T) {
	composed := fixedEvent(t)
	composed.Reason = "café" // é já composto (U+00E9)

	decomposed := fixedEvent(t)
	decomposed.Reason = "café" // e + U+0301 (combining acute)

	a, err := Canonical(composed)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	b, err := Canonical(decomposed)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("NFC não aplicado: formas equivalentes produziram bytes distintos")
	}
}

// Sensibilidade: alterar QUALQUER campo relevante muda os bytes canônicos —
// senão a adulteração passaria despercebida.
func TestCanonicalSensitivity(t *testing.T) {
	base, err := Canonical(fixedEvent(t))
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	mutations := map[string]func(*AuditEvent){
		"action":     func(e *AuditEvent) { e.Action = ActionAuthLogout },
		"outcome":    func(e *AuditEvent) { e.Outcome = Denied },
		"org":        func(e *AuditEvent) { e.OrganizationID = uuid.New() },
		"event_id":   func(e *AuditEvent) { e.EventID = uuid.New() },
		"reason":     func(e *AuditEvent) { e.Reason = "outro" },
		"occurred":   func(e *AuditEvent) { e.OccurredAt = e.OccurredAt.Add(time.Microsecond) },
		"actor_sub":  func(e *AuditEvent) { e.Actor.IdentitySubject = "outro" },
		"target_id":  func(e *AuditEvent) { e.Target.ID = "outro" },
		"context_ip": func(e *AuditEvent) { e.Context.IP = "198.51.100.1" },
		"amr":        func(e *AuditEvent) { e.Context.AuthMethods = []string{"pwd"} },
	}
	for name, mutate := range mutations {
		e := fixedEvent(t)
		mutate(&e)
		got, err := Canonical(e)
		if err != nil {
			t.Fatalf("%s: Canonical: %v", name, err)
		}
		if string(got) == string(base) {
			t.Errorf("mutação %q não alterou os bytes canônicos", name)
		}
	}
}

// occurred_at é truncado a microssegundos (precisão do timestamptz), então
// variações sub-microssegundo NÃO mudam o canônico — o verificador reproduz a
// partir do valor armazenado.
func TestCanonicalOccurredAtMicrosecondPrecision(t *testing.T) {
	e1 := fixedEvent(t)
	e2 := fixedEvent(t)
	e2.OccurredAt = e2.OccurredAt.Add(500 * time.Nanosecond) // sub-micro
	a, _ := Canonical(e1)
	b, _ := Canonical(e2)
	if string(a) != string(b) {
		t.Fatalf("variação sub-microssegundo não deveria mudar o canônico")
	}
}
