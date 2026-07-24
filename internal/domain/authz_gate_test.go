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
	"testing"
)

// A matriz completa do portão fail-closed (INV-6): nenhum erro concede.
func TestDecisionOutcomeMatrix(t *testing.T) {
	boom := errors.New("qualquer falha")
	cases := []struct {
		name        string
		dec         Decision
		err         error
		wantGranted bool
		wantOutcome Outcome
	}{
		{"permitido sem erro", Allow("ok"), nil, true, Allowed},
		{"negado sem erro", DenyDecision("sem relação"), nil, false, Denied},
		{"erro com decisão vazia", Decision{}, boom, false, Failed},
		// O caso crítico: um erro com uma decisão "permitida" obsoleta AINDA nega.
		{"erro DOMINA allowed obsoleto", Allow("stale"), boom, false, Failed},
		{"erro PDP indisponível", Decision{}, ErrPDPUnavailable, false, Failed},
	}
	for _, c := range cases {
		granted, outcome := DecisionOutcome(c.dec, c.err)
		if granted != c.wantGranted || outcome != c.wantOutcome {
			t.Fatalf("%s: granted=%v outcome=%v; queria granted=%v outcome=%v",
				c.name, granted, outcome, c.wantGranted, c.wantOutcome)
		}
	}
}

// O portão e a montagem do evento de auditoria concordam sempre no outcome.
func TestGateAndAuditAgree(t *testing.T) {
	rec := decisionAudit()
	samples := []struct {
		dec Decision
		err error
	}{
		{Allow("via operator"), nil},
		{DenyDecision("sem relação"), nil},
		{Decision{}, ErrPDPUnavailable},
		{Allow("stale"), errors.New("db caiu")},
	}
	for _, s := range samples {
		_, gateOutcome := DecisionOutcome(s.dec, s.err)
		in := BuildDecisionAuditInput(rec, s.dec, s.err)
		if in.Outcome != gateOutcome {
			t.Fatalf("outcome do evento (%v) diverge do portão (%v)", in.Outcome, gateOutcome)
		}
	}
}
