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

import "errors"

// A privileged operation has exactly one of three outcomes. The distinction
// between a *denial* (a decision) and an *error* (a failure) is load-bearing for
// ArchGuard: the audit trail must record which one occurred, and INV-6 requires
// that any failure of a control (PDP, vault, audit) resolves to denial, never to
// permit — there is no fail-open (CLAUDE.md §6, ADR-0005).
type Outcome int

const (
	// Allowed: the operation is permitted.
	Allowed Outcome = iota
	// Denied: the operation is refused by an explicit policy/authorization
	// decision. This is a normal, auditable result — not a fault.
	Denied
	// Failed: a control could not reach a decision (PDP unreachable, vault
	// down, audit not durably written). By INV-6 this MUST be treated as a
	// denial by callers; it is surfaced separately so the cause is auditable.
	Failed
)

func (o Outcome) String() string {
	switch o {
	case Allowed:
		return "allowed"
	case Denied:
		return "denied"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Permitted reports whether the operation may proceed. Only Allowed permits;
// both Denied and Failed do not — the fail-closed rule (INV-6) in one place.
func (o Outcome) Permitted() bool {
	return o == Allowed
}

// ErrDenied is the sentinel for an explicit authorization denial (a decision).
// ErrControlUnavailable is the sentinel for a control failure that, by INV-6,
// callers must treat as denial. Keeping them distinct lets the audit trail
// record the true cause without ever letting a failure become a permit.
var (
	ErrDenied             = errors.New("denied")
	ErrControlUnavailable = errors.New("control unavailable")
)
