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

// Package metrics holds the PROVISIONAL, in-memory implementation of the
// authorization metrics port (domain.AuthzMetrics). It records observations for
// dev/CI and assertions; the real OpenTelemetry exporter (→ VictoriaMetrics) is
// pacote 010. Keeping this behind the domain port means wiring the exporter later
// touches no decision code.
package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// DecisionSample is one recorded PDP decision latency observation.
type DecisionSample struct {
	Duration time.Duration
	Outcome  domain.Outcome
}

// ReconciliationSample is one recorded reconciliation divergence observation.
type ReconciliationSample struct {
	Removed        int
	MissingAlerted int
}

// MemoryAuthzMetrics records observations in memory for inspection in tests and
// dev. It is safe for concurrent use.
type MemoryAuthzMetrics struct {
	mu        sync.Mutex
	decisions []DecisionSample
	recons    []ReconciliationSample
}

// NewMemoryAuthzMetrics builds an empty in-memory metrics sink.
func NewMemoryAuthzMetrics() *MemoryAuthzMetrics { return &MemoryAuthzMetrics{} }

// ObserveDecisionLatency records a decision latency sample.
func (m *MemoryAuthzMetrics) ObserveDecisionLatency(_ context.Context, d time.Duration, outcome domain.Outcome) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, DecisionSample{Duration: d, Outcome: outcome})
}

// ObserveReconciliation records a reconciliation divergence sample.
func (m *MemoryAuthzMetrics) ObserveReconciliation(_ context.Context, removed, missingAlerted int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recons = append(m.recons, ReconciliationSample{Removed: removed, MissingAlerted: missingAlerted})
}

// Decisions returns a copy of the recorded decision samples.
func (m *MemoryAuthzMetrics) Decisions() []DecisionSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]DecisionSample(nil), m.decisions...)
}

// Reconciliations returns a copy of the recorded reconciliation samples.
func (m *MemoryAuthzMetrics) Reconciliations() []ReconciliationSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ReconciliationSample(nil), m.recons...)
}

var _ domain.AuthzMetrics = (*MemoryAuthzMetrics)(nil)
