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
	"errors"
	"time"
)

// PolicyDecisionPoint is the domain's port to the fine-grained authorization
// engine (OpenFGA, pacote 007 — ADR-0005). It is deliberately vendor-neutral:
// NO OpenFGA SDK type crosses this boundary (RFC-0004 §7, INV-3), so the engine
// can be swapped for SpiceDB without touching the domain. The four operations
// mirror the Zanzibar-style surface the RFC names — `check`, `listObjects`,
// `write`, `read`:
//
//   - Check answers one authorization question (the decision path).
//   - ListObjects is the reverse query for access-review campaigns (T-014).
//   - Write applies tuple mutations the outbox publisher derives from the DB.
//   - Read fetches tuples for reconciliation and debugging.
//
// FAIL-CLOSED CONTRACT (INV-6 / RFC-0004 §6): a Check that cannot reach a
// verdict — PDP unreachable, timeout, malformed model — MUST return a non-nil
// error, and every caller MUST treat that error as a DENIAL. A denial the engine
// actually computed is Decision{Allowed:false}, err==nil. The two are distinct on
// the wire precisely so the audit trail can tell an authorization `denied` from
// an infrastructure `error` (T-013); no implementation may collapse an error into
// an allow.
type PolicyDecisionPoint interface {
	// Check evaluates a single authorization question. A computed refusal is
	// (Decision{Allowed:false}, nil); an infrastructure failure is (_, error) and
	// the caller denies.
	Check(ctx context.Context, req CheckRequest) (Decision, error)
	// ListObjects returns the tenant-qualified ids of every object of req.Type on
	// which req.User holds req.Relation — the reverse query behind "who has access
	// to what" (access review, T-014).
	ListObjects(ctx context.Context, req ListObjectsRequest) ([]string, error)
	// Write applies a batch of tuple additions/removals idempotently. It is driven
	// exclusively by the transactional outbox (RFC-0004 §4) — never by hand.
	Write(ctx context.Context, updates []TupleUpdate) error
	// Read returns the tuples matching filter, used by reconciliation (T-008) and
	// bootstrap/replay (T-009).
	Read(ctx context.Context, filter TupleFilter) ([]RelationTuple, error)
}

// RelationTuple is one edge of the authorization graph: subject User holds
// Relation over Object. All three are OPAQUE, tenant-qualified strings — the
// domain does not parse them here (the qualifier lives in T-003) — following the
// engine's convention:
//
//	User:     "membership:<uuid>" or "group:<uuid>#member"
//	Relation: "operator", "can_open_privileged_session", "has_active_grant", …
//	Object:   "org:<uuid>/asset:<uuid>" or "org:<uuid>/asset_group:<uuid>"
//
// Keeping the object tenant-qualified is what prevents a tuple from crossing
// organizations (INV-5 / RFC-0004 §3).
type RelationTuple struct {
	User     string
	Relation string
	Object   string
}

// Valid reports whether all three components are present. A tuple missing any
// component is never written or checked.
func (t RelationTuple) Valid() bool {
	return t.User != "" && t.Relation != "" && t.Object != ""
}

// TupleOp is the kind of mutation a TupleUpdate carries.
type TupleOp string

const (
	// TupleWrite adds a tuple (idempotent: re-adding an existing tuple is a no-op).
	TupleWrite TupleOp = "write"
	// TupleDelete removes a tuple (idempotent: deleting an absent tuple is a no-op).
	TupleDelete TupleOp = "delete"
)

// Valid reports whether o is a defined operation.
func (o TupleOp) Valid() bool { return o == TupleWrite || o == TupleDelete }

// TupleUpdate is a single idempotent mutation the publisher applies to the store.
type TupleUpdate struct {
	Op    TupleOp
	Tuple RelationTuple
}

// TupleFilter selects tuples for Read. Empty fields are wildcards; at least one
// field SHOULD be set to bound the scan (an all-wildcard read is a full store
// dump, used only by replay).
type TupleFilter struct {
	User     string
	Relation string
	Object   string
}

// CheckContext carries the contextual conditions a PAM decision depends on beyond
// the static graph edge (RFC-0004 §5): the proven assurance level, the instant of
// evaluation (so a grant's temporal window is judged against a trusted clock, not
// the store's), and the request origin. It travels into the engine as the
// `context` of the check and is echoed into the audit justification.
type CheckContext struct {
	// ACR is the proven assurance level of the session making the request (L1/L2/L3,
	// ADR-0010). A relation may require a minimum ACR (e.g. privileged session ⇒ L2+).
	ACR string
	// EvaluatedAt is the decision instant. Temporal windows (grants, break-glass)
	// are judged against it — the concession expires in the graph at this instant,
	// not merely in a cleanup job (RFC-0004 §3).
	EvaluatedAt time.Time
	// Origin is an opaque, non-personal label for where the request came from
	// (e.g. a broker id). Never carries personal data (INV-7).
	Origin string
}

// CheckRequest is one authorization question: does Tuple.User hold Tuple.Relation
// over Tuple.Object, given Context?
type CheckRequest struct {
	Tuple   RelationTuple
	Context CheckContext
}

// Valid reports whether the request is well-formed enough to evaluate. A malformed
// request is refused before it reaches the engine (and, per the fail-closed
// contract, a refusal to evaluate is a denial).
func (r CheckRequest) Valid() bool { return r.Tuple.Valid() }

// ListObjectsRequest is the reverse query: every Object of Type on which User
// holds Relation. Used by access-review campaigns (T-014 / spec "Campanha de
// revisão").
type ListObjectsRequest struct {
	User     string
	Relation string
	// Type is the object type to enumerate (e.g. "asset"), tenant-qualified by the
	// caller so the scan stays within one organization.
	Type string
}

// Valid reports whether the reverse query is well-formed.
func (r ListObjectsRequest) Valid() bool {
	return r.User != "" && r.Relation != "" && r.Type != ""
}

// Decision is the engine's COMPUTED verdict for a Check. It is only ever
// constructed from a verdict the engine actually reached: an infrastructure
// failure is reported as an error from Check, never as Decision{Allowed:false}.
// Justification captures the resolution path (direct, inherited, by grant) and is
// attached verbatim to the audit event (T-012 / RFC-0004 §5).
type Decision struct {
	// Allowed is the verdict: true only if the engine resolved the relation.
	Allowed bool
	// Justification is a short, non-personal explanation of WHY (e.g.
	// "operator from parent asset_group:<id>"), recorded in the audit trail. It
	// must never carry personal data or secrets (INV-7).
	Justification string
}

// Allow builds an allowed decision with a justification.
func Allow(justification string) Decision {
	return Decision{Allowed: true, Justification: justification}
}

// Deny builds a computed denial with a justification.
func DenyDecision(justification string) Decision {
	return Decision{Allowed: false, Justification: justification}
}

// Denied reports whether the decision refuses access. It is the inverse of
// Allowed, named for the audit vocabulary (a Check error is a separate `error`
// outcome, distinguished from this `denied`).
func (d Decision) Denied() bool { return !d.Allowed }

// ErrPDPUnavailable is the sentinel a PDP adapter returns when it cannot reach a
// verdict (engine unreachable, timeout). Callers translate it to a denial with an
// audit outcome of `error` (distinct from a computed `denied`), never to an allow
// (INV-6 / RFC-0004 §6). There is no configuration that turns this into an allow.
var ErrPDPUnavailable = errors.New("pdp: ponto de decisão indisponível — acesso negado (fail-closed)")
