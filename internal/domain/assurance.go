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
	"fmt"
	"sort"
	"time"
)

// Freshness windows per level (ADR-0010): L1 imposes no age limit (a valid
// session suffices); L2 accepts a strong factor proven within a generous window;
// L3 demands a RECENT phishing-resistant reauthentication — a short window,
// regardless of how old the session is. These are the platform defaults; the
// per-organization policy (T-010) may only TIGHTEN them, never loosen.
const (
	freshnessL2Window = 12 * time.Hour
	freshnessL3Window = 5 * time.Minute
)

// This file adds the assurance-level POLICY (what each level demands of a
// session) and the API-operation classification CATALOG (INV-8) on top of the
// AssuranceLevel type defined in audit_event.go. It deliberately reuses that
// type — there is ONE notion of L1/L2/L3 in the domain, shared by the audit
// action catalog and by the step-up middleware, so a verb and its endpoint can
// never disagree on what it costs.

// Valid reports whether l is a defined assurance level. The zero value ("") is
// NOT valid: an operation cannot silently default to unclassified (INV-8).
func (l AssuranceLevel) Valid() bool {
	switch l {
	case L1, L2, L3:
		return true
	default:
		return false
	}
}

// RequiredAAL is the authenticator assurance level the operation level demands:
// L1→AAL1, L2→AAL2, L3→AAL3 (ADR-0010). An undefined level maps to AAL3 —
// fail-closed: an unrecognized classification demands the STRONGEST proof, never
// the weakest.
func (l AssuranceLevel) RequiredAAL() AAL {
	switch l {
	case L1:
		return AAL1
	case L2:
		return AAL2
	default: // L3 and any unrecognized value
		return AAL3
	}
}

// RequiresPhishingResistant reports whether the operation level demands a
// phishing-resistant factor (WebAuthn). Only L3 does — but, fail-closed, an
// unrecognized level is treated as if it does.
func (l AssuranceLevel) RequiresPhishingResistant() bool {
	return l != L1 && l != L2
}

// RequiresFreshness reports whether the level constrains how recently the
// session authenticated. L1 does not (a valid session is enough); L2 and L3 do —
// and, fail-closed, an unrecognized level does too.
func (l AssuranceLevel) RequiresFreshness() bool {
	return l != L1
}

// FreshnessWindow is the maximum age of the session's authentication the level
// tolerates. L1 has no window (RequiresFreshness is false); L2 a generous one;
// L3 a short one. An unrecognized level gets the SHORTEST window — fail-closed.
func (l AssuranceLevel) FreshnessWindow() time.Duration {
	switch l {
	case L1:
		return 0 // unused: L1 imposes no freshness constraint
	case L2:
		return freshnessL2Window
	default: // L3 and any unrecognized value
		return freshnessL3Window
	}
}

// Fresh reports whether a session that authenticated at authTime is recent enough
// for this level at instant now. It is fail-closed: a level that requires
// freshness with a zero or future authTime is NOT fresh, and the age must be
// within (not merely at) the window. L1 is always fresh.
func (l AssuranceLevel) Fresh(authTime, now time.Time) bool {
	if !l.RequiresFreshness() {
		return true
	}
	if authTime.IsZero() {
		return false
	}
	age := now.Sub(authTime)
	if age < 0 {
		return false // authTime in the future — a clock/forgery anomaly, deny.
	}
	return age <= l.FreshnessWindow()
}

// Satisfies reports whether a proven assurance (the session's AAL and whether it
// was obtained with a phishing-resistant factor) meets this operation level. It
// checks assurance level AND phishing resistance; FRESHNESS is a separate axis
// (Fresh), which the guard composes with this.
func (l AssuranceLevel) Satisfies(provenAAL AAL, phishingResistant bool) bool {
	if !provenAAL.AtLeast(l.RequiredAAL()) {
		return false
	}
	if l.RequiresPhishingResistant() && !phishingResistant {
		return false
	}
	return true
}

// Operation is one classified API operation: a STABLE identifier (never the
// display route, which changes), the level it requires, and a human description
// for the catalog. The identifier is what the router/middleware keys on. This is
// the API-endpoint axis of INV-8, distinct from (but sharing the level type of)
// the audit-verb actionCatalog: an endpoint enforces a level before dispatch, a
// verb records the level it was performed at.
type Operation struct {
	ID          string
	Level       AssuranceLevel
	Description string
}

// Errors of the operation catalog.
var (
	// ErrOperationNotClassified is returned when the catalog is asked for an
	// operation it does not hold. It is a DENIAL, not a lookup miss to shrug off:
	// an unclassified operation must be refused (INV-8 fail-closed).
	ErrOperationNotClassified = errors.New("assurance: operação sem classificação de nível de garantia")
	// ErrOperationInvalid is returned when registering a malformed operation
	// (empty id or undefined level).
	ErrOperationInvalid = errors.New("assurance: operação inválida")
	// ErrOperationDuplicate is returned when registering an id already present —
	// a double classification is ambiguous and must be caught at wiring time.
	ErrOperationDuplicate = errors.New("assurance: operação já classificada")
)

// OperationCatalog is the single source of truth for API-operation
// classification. It is populated once at startup (and, in the invariant test,
// from the router) and then only read. A lookup for an unregistered operation is
// a denial, so the catalog cannot be the reason a privileged path runs
// unprotected.
type OperationCatalog struct {
	ops map[string]Operation
}

// NewOperationCatalog builds an empty catalog.
func NewOperationCatalog() *OperationCatalog {
	return &OperationCatalog{ops: make(map[string]Operation)}
}

// Register adds an operation, refusing a malformed one (ErrOperationInvalid) or a
// duplicate id (ErrOperationDuplicate). Registration is the ONLY way to classify;
// there is no implicit level.
func (c *OperationCatalog) Register(op Operation) error {
	if op.ID == "" {
		return fmt.Errorf("%w: id vazio", ErrOperationInvalid)
	}
	if !op.Level.Valid() {
		return fmt.Errorf("%w: nível %q de %q", ErrOperationInvalid, op.Level, op.ID)
	}
	if _, exists := c.ops[op.ID]; exists {
		return fmt.Errorf("%w: %q", ErrOperationDuplicate, op.ID)
	}
	c.ops[op.ID] = op
	return nil
}

// Level returns the required level of an operation, or ErrOperationNotClassified
// if it was never registered — the fail-closed lookup the middleware uses.
func (c *OperationCatalog) Level(id string) (AssuranceLevel, error) {
	op, ok := c.ops[id]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrOperationNotClassified, id)
	}
	return op.Level, nil
}

// Lookup returns the full operation record, or false if unregistered.
func (c *OperationCatalog) Lookup(id string) (Operation, bool) {
	op, ok := c.ops[id]
	return op, ok
}

// IDs returns the classified operation ids in sorted order — the set the
// completeness invariant (T-017) compares against the router's operations.
func (c *OperationCatalog) IDs() []string {
	ids := make([]string, 0, len(c.ops))
	for id := range c.ops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// InsufficientAssuranceError is the SPECIFIC denial the assurance middleware
// returns when a session does not meet an operation's level. It names what the
// client must do to proceed — the required level and the acr to reach — so the
// HTTP layer can emit a step-up CHALLENGE (RFC 9470: acr_values) instead of a
// bare 403. It is a denial (a decision), never a system error: the middleware
// distinguishes it from an infrastructure failure so the audit trail records the
// right outcome.
type InsufficientAssuranceError struct {
	// Operation is the classified operation id that was refused.
	Operation string
	// Required is the level the operation demands.
	Required AssuranceLevel
	// RequiredACR is the acr value the client must reach (the required level's
	// AAL, e.g. "aal3") — the acr_values of the step-up challenge.
	RequiredACR string
	// NeedsPhishingResistant is true when the operation (L3) also demands a
	// phishing-resistant factor, so the client knows a TOTP step-up will not do.
	NeedsPhishingResistant bool
	// ProvenACR is the acr the session currently holds ("" if none) — for the
	// error message and audit, never to weaken the decision.
	ProvenACR string
	// Stale is true when the session met the level's AAL/phishing requirement but
	// its authentication is too OLD for the level's freshness window — the client
	// must REAUTHENTICATE (step-up) even though it already holds the right factor.
	Stale bool
}

func (e *InsufficientAssuranceError) Error() string {
	if e.Stale {
		return fmt.Sprintf("assurance: reautenticação exigida para %q: %s (acr %s) requer autenticação recente",
			e.Operation, e.Required, e.RequiredACR)
	}
	return fmt.Sprintf("assurance: garantia insuficiente para %q: exige %s (acr %s), sessão tem acr %q",
		e.Operation, e.Required, e.RequiredACR, e.ProvenACR)
}

// AssuranceGuard enforces operation classification against a session's proven
// assurance. It is the decision core the HTTP middleware (T-007) wraps; freshness
// (T-008) composes onto it. It holds the catalog read-only.
type AssuranceGuard struct {
	catalog *OperationCatalog
}

// NewAssuranceGuard builds the guard over a populated catalog.
func NewAssuranceGuard(catalog *OperationCatalog) *AssuranceGuard {
	return &AssuranceGuard{catalog: catalog}
}

// Authorize decides whether the session may perform the operation. It is
// fail-closed on every axis:
//
//   - an UNCLASSIFIED operation is refused with ErrOperationNotClassified (a
//     missing level is a hole, never an implicit allow — INV-8);
//   - a nil or non-active session cannot carry proven assurance, so it is
//     refused with an InsufficientAssuranceError naming the required acr;
//   - a session below the required level (AAL or phishing resistance) is refused
//     with the same specific error.
//
// It returns nil only when the session meets the operation's level AND its
// authentication is fresh enough for that level (evaluated at now, supplied by
// the caller so the domain stays clock-free). A session at the right level but
// too OLD is refused with Stale set — the "L3 with an old session" denial.
func (g *AssuranceGuard) Authorize(operationID string, s *AuthSession, now time.Time) error {
	level, err := g.catalog.Level(operationID)
	if err != nil {
		return err // ErrOperationNotClassified — deny, never allow.
	}
	insufficient := func(provenACR string, stale bool) error {
		return &InsufficientAssuranceError{
			Operation:              operationID,
			Required:               level,
			RequiredACR:            string(level.RequiredAAL()),
			NeedsPhishingResistant: level.RequiresPhishingResistant(),
			ProvenACR:              provenACR,
			Stale:                  stale,
		}
	}
	if s == nil || s.Status != SessionActive {
		return insufficient("", false)
	}
	if !level.Satisfies(s.ProvenAAL, s.PhishingResistant()) {
		return insufficient(s.ACR(), false)
	}
	// The factor is strong enough; now it must also be RECENT enough. A stale
	// authentication demands step-up even at the correct level.
	if !level.Fresh(s.AuthTime, now) {
		return insufficient(s.ACR(), true)
	}
	return nil
}
