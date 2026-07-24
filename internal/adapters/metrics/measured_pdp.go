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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// MeasuredPDP decorates a domain.PolicyDecisionPoint to record the latency and
// outcome of every Check (T-019). Timing lives in a decorator, not inside the PDP,
// so the decision logic stays free of the clock and the metrics dependency, and
// the measurement is opt-in via composition. The classification uses the same
// fail-closed gate the callers use (domain.DecisionOutcome), so the recorded
// outcome always matches the effect.
type MeasuredPDP struct {
	inner   domain.PolicyDecisionPoint
	metrics domain.AuthzMetrics
	now     func() time.Time
}

// NewMeasuredPDP wraps inner, recording Check latency/outcome to metrics. now
// supplies the clock (injected for determinism); pass time.Now in production.
func NewMeasuredPDP(inner domain.PolicyDecisionPoint, m domain.AuthzMetrics, now func() time.Time) *MeasuredPDP {
	return &MeasuredPDP{inner: inner, metrics: m, now: now}
}

// Check times the underlying decision and records latency + outcome.
func (p *MeasuredPDP) Check(ctx context.Context, req domain.CheckRequest) (domain.Decision, error) {
	start := p.now()
	dec, err := p.inner.Check(ctx, req)
	_, outcome := domain.DecisionOutcome(dec, err)
	p.metrics.ObserveDecisionLatency(ctx, p.now().Sub(start), outcome)
	return dec, err
}

// ListObjects passes through (listings are not the privileged decision path).
func (p *MeasuredPDP) ListObjects(ctx context.Context, req domain.ListObjectsRequest) ([]string, error) {
	return p.inner.ListObjects(ctx, req)
}

// Write passes through.
func (p *MeasuredPDP) Write(ctx context.Context, updates []domain.TupleUpdate) error {
	return p.inner.Write(ctx, updates)
}

// Read passes through.
func (p *MeasuredPDP) Read(ctx context.Context, filter domain.TupleFilter) ([]domain.RelationTuple, error) {
	return p.inner.Read(ctx, filter)
}

var _ domain.PolicyDecisionPoint = (*MeasuredPDP)(nil)
