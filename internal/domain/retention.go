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
	"time"
)

// Audit retention by ARCHIVAL, never selective deletion (pacote 010, T-020 /
// design 010 §LGPD: "expiração leva a arquivamento de partição selada; nunca
// deleção seletiva"). This policy decides WHICH sealed time partition is past its
// retention and should be archived (the archival + audited restore mechanism is
// the PartitionArchiver, pacote 003; the scheduling is deploy). No event is ever
// deleted — a whole sealed partition is moved to archival storage with its seals.

// ErrInvalidRetention is returned when a retention period is not positive.
var ErrInvalidRetention = errors.New("retention: período de retenção deve ser positivo")

// RetentionPolicy is the online-retention configuration for the audit trail.
type RetentionPolicy struct {
	// Period is how long a sealed partition is kept online before archival
	// (e.g. 365 days). The final value is the controller's (the client) decision.
	Period time.Duration
}

// NewRetentionPolicy validates and builds a policy.
func NewRetentionPolicy(period time.Duration) (RetentionPolicy, error) {
	if period <= 0 {
		return RetentionPolicy{}, ErrInvalidRetention
	}
	return RetentionPolicy{Period: period}, nil
}

// Cutoff is the instant before which data is past retention at now.
func (p RetentionPolicy) Cutoff(now time.Time) time.Time {
	return now.Add(-p.Period)
}

// PastRetention reports whether a partition whose upper bound is `to` is FULLY past
// retention at now — its entire range predates the cutoff, so the partition may be
// archived. A partition still within (even partly) the retention window is kept.
func (p RetentionPolicy) PastRetention(to, now time.Time) bool {
	return !to.After(p.Cutoff(now))
}
