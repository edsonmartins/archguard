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
	"context"
	"time"
)

// AuthzMetrics records the observable signals of the authorization plane
// (RFC-0004 §8 / ADR-0013): how long decisions take and how far the PDP store has
// drifted from the source of truth. It is the SEAM — the OpenTelemetry exporter is
// pacote 010; a provisional in-memory implementation serves dev/CI. It carries NO
// personal data, only durations, counts and the outcome enum (INV-7).
type AuthzMetrics interface {
	// ObserveDecisionLatency records the wall-clock duration of one PDP Check and
	// its outcome (Allowed/Denied/Failed), so slow decisions and a rising error
	// rate are visible on the dashboards.
	ObserveDecisionLatency(ctx context.Context, d time.Duration, outcome Outcome)
	// ObserveReconciliation records what one reconciliation pass found for a tenant:
	// how many extra tuples were removed (restrictive, automatic) and how many
	// missing were alerted (amplifying, human review). A persistently non-zero
	// divergence is an operational signal that the projection is drifting.
	ObserveReconciliation(ctx context.Context, removed, missingAlerted int)
}

// NopAuthzMetrics discards observations — the safe default when metrics are not
// wired, so a caller can always hold a non-nil AuthzMetrics.
type NopAuthzMetrics struct{}

func (NopAuthzMetrics) ObserveDecisionLatency(context.Context, time.Duration, Outcome) {}
func (NopAuthzMetrics) ObserveReconciliation(context.Context, int, int)                {}

var _ AuthzMetrics = NopAuthzMetrics{}
