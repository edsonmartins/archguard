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
	"sync"
	"time"

	"github.com/google/uuid"
)

// Credential-stuffing detection parameters (spec "Padrão distribuído"). When ONE
// origin drives failed logins against many DISTINCT identities inside a short
// window, that is the distributed pattern of a stuffing attack — distinct from a
// single account being brute-forced (which the per-identity throttle, T-014,
// handles). stuffingDistinctThreshold identities from one origin within
// stuffingWindow raises the alert.
const (
	stuffingWindow            = 5 * time.Minute
	stuffingDistinctThreshold = 10
)

// StuffingDetector tracks, per ORIGIN, the distinct identities a failed login has
// targeted within a sliding window, and reports when an origin crosses the
// stuffing threshold. It is an in-process detector — the seam for the durable,
// cross-replica version that lives in observability (pacote 010). The origin is
// an OPAQUE key the caller supplies (a hash of the source address, never a raw
// IP here — access context belongs to the audit event, RFC-0003, not to this
// map), so no personal data is retained (INV-7).
//
// It is safe for concurrent use.
type StuffingDetector struct {
	mu sync.Mutex
	// seen maps an origin key to the last-seen time of each distinct identity it
	// targeted; entries older than the window are pruned on each observation.
	seen map[string]map[uuid.UUID]time.Time
	// alerted marks origins already alerted in the current window, so one attack
	// raises one alert, not one per attempt over the threshold.
	alerted map[string]bool
}

// NewStuffingDetector builds an empty detector.
func NewStuffingDetector() *StuffingDetector {
	return &StuffingDetector{
		seen:    make(map[string]map[uuid.UUID]time.Time),
		alerted: make(map[string]bool),
	}
}

// Observe records a failed authentication for identityID coming from originKey at
// now, and reports whether this observation raises a credential-stuffing alert.
// It returns the alert flag and the current count of distinct identities the
// origin has targeted in the window. The alert fires ONCE per origin per window
// (on the observation that first reaches the threshold), so the caller — which
// wires the Alerter — is not spammed.
func (d *StuffingDetector) Observe(originKey string, identityID uuid.UUID, now time.Time) (alert bool, distinct int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ids := d.seen[originKey]
	if ids == nil {
		ids = make(map[uuid.UUID]time.Time)
		d.seen[originKey] = ids
	}
	// Prune identities last seen before the window.
	cutoff := now.Add(-stuffingWindow)
	for id, ts := range ids {
		if ts.Before(cutoff) {
			delete(ids, id)
		}
	}
	// If the window has fully drained, this origin may alert again.
	if len(ids) == 0 {
		d.alerted[originKey] = false
	}
	ids[identityID] = now
	distinct = len(ids)

	if distinct >= stuffingDistinctThreshold && !d.alerted[originKey] {
		d.alerted[originKey] = true
		return true, distinct
	}
	return false, distinct
}
