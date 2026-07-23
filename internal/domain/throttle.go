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

	"github.com/google/uuid"
)

// Progressive-lockout parameters (ADR-0010 / spec "Tentativas repetidas"). No
// lockout applies until throttleThreshold consecutive failures; from there the
// lockout grows geometrically (throttleBase doubled per extra failure) up to
// throttleMax. Growing the penalty makes brute force impractical while a couple
// of honest typos cost nothing.
const (
	throttleThreshold = 5
	throttleBase      = 30 * time.Second
	throttleMax       = 1 * time.Hour
)

// Throttle is the per-subject authentication-failure state used for progressive
// lockout: how many CONSECUTIVE failures have occurred and until when the subject
// is locked out. It is the domain value the login flow reads before accepting an
// attempt and updates after — the store persists it (T-014). The zero value is a
// clean subject (no failures, not locked).
type Throttle struct {
	Failures    int
	LockedUntil time.Time
}

// Locked reports whether the subject is currently locked out at now — the gate
// the login flow checks before even validating the credential. Fail-closed by
// construction: a lockout in the future denies.
func (t Throttle) Locked(now time.Time) bool {
	return now.Before(t.LockedUntil)
}

// RecordFailure returns the state after one more failed attempt at now: the
// failure count rises and, once it reaches the threshold, a progressively longer
// lockout is applied from now.
func (t Throttle) RecordFailure(now time.Time) Throttle {
	next := Throttle{Failures: t.Failures + 1}
	if d := lockoutFor(next.Failures); d > 0 {
		next.LockedUntil = now.Add(d)
	}
	return next
}

// RecordSuccess resets the state — a genuine login clears the streak so honest
// users are never punished for a later typo.
func (t Throttle) RecordSuccess() Throttle {
	return Throttle{}
}

// lockoutFor is the progressive lockout duration for a given consecutive-failure
// count: zero below the threshold, then throttleBase doubled per extra failure,
// capped at throttleMax.
func lockoutFor(failures int) time.Duration {
	if failures < throttleThreshold {
		return 0
	}
	d := throttleBase
	for i := throttleThreshold; i < failures; i++ {
		d *= 2
		if d >= throttleMax {
			return throttleMax
		}
	}
	return d
}

// ThrottleStore persists per-identity authentication-failure state. Get returns
// the current state (the zero Throttle for a clean subject); Save upserts it. A
// STORE FAILURE is an error the login flow denies on (INV-6): the throttle can
// never be bypassed by an unreadable state.
type ThrottleStore interface {
	Get(ctx context.Context, identityID uuid.UUID) (Throttle, error)
	Save(ctx context.Context, identityID uuid.UUID, t Throttle) error
}
