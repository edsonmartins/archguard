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

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// fixedPDP returns a preset decision/error, ignoring the request.
type fixedPDP struct {
	dec domain.Decision
	err error
}

func (p fixedPDP) Check(context.Context, domain.CheckRequest) (domain.Decision, error) {
	return p.dec, p.err
}
func (p fixedPDP) ListObjects(context.Context, domain.ListObjectsRequest) ([]string, error) {
	return nil, nil
}
func (p fixedPDP) Write(context.Context, []domain.TupleUpdate) error { return nil }
func (p fixedPDP) Read(context.Context, domain.TupleFilter) ([]domain.RelationTuple, error) {
	return nil, nil
}

// stepClock advances by a fixed step on each read, so start/end differ.
type stepClock struct {
	t    time.Time
	step time.Duration
}

func (c *stepClock) now() time.Time {
	c.t = c.t.Add(c.step)
	return c.t
}

// A latência e o outcome de cada Check são registrados; o outcome casa com o
// portão fail-closed (allowed/denied/error).
func TestMeasuredPDPRecordsLatencyAndOutcome(t *testing.T) {
	cases := []struct {
		name    string
		dec     domain.Decision
		err     error
		outcome domain.Outcome
	}{
		{"permitido", domain.Allow("ok"), nil, domain.Allowed},
		{"negado", domain.DenyDecision("sem relação"), nil, domain.Denied},
		{"erro", domain.Decision{}, domain.ErrPDPUnavailable, domain.Failed},
	}
	for _, c := range cases {
		sink := NewMemoryAuthzMetrics()
		clk := &stepClock{t: time.Unix(0, 0), step: 5 * time.Millisecond}
		pdp := NewMeasuredPDP(fixedPDP{dec: c.dec, err: c.err}, sink, clk.now)

		_, _ = pdp.Check(context.Background(), domain.CheckRequest{
			Tuple: domain.RelationTuple{User: "org:o1/membership:m", Relation: domain.RelCanOpenSession, Object: "org:o1/asset:a"},
		})

		samples := sink.Decisions()
		if len(samples) != 1 {
			t.Fatalf("%s: esperava 1 amostra, veio %d", c.name, len(samples))
		}
		if samples[0].Outcome != c.outcome {
			t.Fatalf("%s: outcome %v, esperado %v", c.name, samples[0].Outcome, c.outcome)
		}
		if samples[0].Duration <= 0 {
			t.Fatalf("%s: latência medida deveria ser positiva, veio %v", c.name, samples[0].Duration)
		}
	}
}

// A divergência de reconciliação é registrada com as contagens.
func TestObserveReconciliation(t *testing.T) {
	sink := NewMemoryAuthzMetrics()
	sink.ObserveReconciliation(context.Background(), 3, 2)
	recs := sink.Reconciliations()
	if len(recs) != 1 || recs[0].Removed != 3 || recs[0].MissingAlerted != 2 {
		t.Fatalf("amostra de reconciliação inesperada: %+v", recs)
	}
}

// O Nop descarta sem quebrar (default seguro).
func TestNopAuthzMetrics(t *testing.T) {
	var m domain.AuthzMetrics = domain.NopAuthzMetrics{}
	m.ObserveDecisionLatency(context.Background(), time.Second, domain.Allowed)
	m.ObserveReconciliation(context.Background(), 1, 1)
}
