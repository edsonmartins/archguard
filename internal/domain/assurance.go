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

// Satisfies reports whether a proven assurance (the session's AAL and whether it
// was obtained with a phishing-resistant factor) meets this operation level. It
// checks assurance level AND phishing resistance; FRESHNESS is layered on by the
// step-up middleware (T-008), which composes this with a freshness check.
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
