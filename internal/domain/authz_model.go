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

import "time"

// This file is the ArchGuard fine-grained authorization MODEL (T-002 / RFC-0004
// §3): the graph types, their relations, the inheritance rewrites and the
// temporal condition. It is a pure, vendor-neutral encoding of the same model an
// OpenFGA store would hold — the engine is a projection of THIS (ADR-0005). No
// framework, no SDK, no ORM (INV-3). The resolver that evaluates it lives in
// authz_graph.go.

// ObjectType names a node type of the authorization graph (RFC-0004 §3).
type ObjectType string

const (
	TypeOrganization ObjectType = "organization"
	TypeMembership   ObjectType = "membership"
	TypeGroup        ObjectType = "group"
	TypeAssetGroup   ObjectType = "asset_group"
	TypeAsset        ObjectType = "asset"
	TypeAccessPolicy ObjectType = "access_policy"
)

// Relation names used across the model. Referencing them by constant keeps the
// model, the resolver and the projection from drifting on a typo.
const (
	RelParent                   = "parent"
	RelOwner                    = "owner"
	RelOperator                 = "operator"
	RelAuditor                  = "auditor"
	RelMember                   = "member"
	RelHasActiveGrant           = "has_active_grant"
	RelCanOpenSession           = "can_open_session"
	RelCanOpenPrivilegedSession = "can_open_privileged_session"
)

// ConditionValidWindow is the temporal condition gating a grant relation: the
// tuple confers access only while the decision instant lies within its window.
// This is what makes a concession EXPIRE IN THE GRAPH (RFC-0004 §3 / spec
// "Concessão expirada") rather than only in a cleanup job.
const ConditionValidWindow = "valid_window"

// ValidityWindow is the [NotBefore, ExpiresAt) interval a conditioned tuple
// carries. Access holds only strictly within it.
type ValidityWindow struct {
	NotBefore time.Time
	ExpiresAt time.Time
}

// Contains reports whether at falls within [NotBefore, ExpiresAt).
func (w ValidityWindow) Contains(at time.Time) bool {
	return !at.Before(w.NotBefore) && at.Before(w.ExpiresAt)
}

// rewriteKind is the operator that computes a relation's effective user set —
// the subset of the Zanzibar rewrite algebra this model uses.
type rewriteKind int

const (
	// rwThis: users assigned by a direct tuple (object, relation, subject),
	// optionally gated by a condition. The subject may be a concrete ref or a
	// userset ("group:g#member").
	rwThis rewriteKind = iota
	// rwComputed: defer to another relation on the SAME object (e.g.
	// can_open_session ⊇ operator).
	rwComputed
	// rwTTU (tuple-to-userset): "<relation> from <tupleset>" — hop through the
	// tupleset relation on this object to each reached object and evaluate
	// <relation> there. This is inheritance up the asset hierarchy
	// ("operator from parent").
	rwTTU
	// rwUnion: related via ANY child (or).
	rwUnion
	// rwIntersection: related via ALL children (and).
	rwIntersection
)

// rewrite is one node of a relation's definition tree.
type rewrite struct {
	kind rewriteKind
	// relation: rwComputed → the relation on this object; rwTTU → the relation to
	// evaluate on each reached object.
	relation string
	// tupleset: rwTTU → the relation on THIS object to hop through (e.g. "parent").
	tupleset string
	// condition: rwThis → the name of the condition gating direct tuples ("" ⇒
	// unconditional). An unknown condition, or a conditioned tuple with no window,
	// fails closed (grants nothing).
	condition string
	// children: rwUnion / rwIntersection operands.
	children []rewrite
}

// rewrite constructors — kept tiny so the model below reads like the RFC DSL.
func direct() rewrite                { return rewrite{kind: rwThis} }
func directCond(cond string) rewrite { return rewrite{kind: rwThis, condition: cond} }
func computed(rel string) rewrite    { return rewrite{kind: rwComputed, relation: rel} }
func from(tupleset, rel string) rewrite {
	return rewrite{kind: rwTTU, tupleset: tupleset, relation: rel}
}
func union(rs ...rewrite) rewrite        { return rewrite{kind: rwUnion, children: rs} }
func intersection(rs ...rewrite) rewrite { return rewrite{kind: rwIntersection, children: rs} }

// authzModel maps each object type's relations to their rewrite definitions.
type authzModel struct {
	relations map[ObjectType]map[string]rewrite
}

// archGuardModel is THE authorization model (RFC-0004 §3). Editing it is a
// governed change — it is the single source of the graph semantics the whole PDP
// obeys, and the projection to any external engine must mirror it exactly.
//
//	type asset
//	  define parent: [asset_group]
//	  define owner: [membership]
//	  define operator: [membership, group#member] or operator from parent
//	  define auditor:  [membership, group#member] or auditor  from parent
//	  define can_open_session: operator or owner
//	  define has_active_grant: [membership with valid_window]
//	  define can_open_privileged_session: can_open_session and has_active_grant
//
// asset_group carries the same operator/auditor with its own parent, so
// inheritance chains through nested groups (cluster → group → asset). group has
// member. organization/membership/access_policy are node types with no rewrite of
// their own here (they appear as subjects/objects of the relations above).
var archGuardModel = authzModel{
	relations: map[ObjectType]map[string]rewrite{
		TypeAsset: {
			RelParent:                   direct(),
			RelOwner:                    direct(),
			RelOperator:                 union(direct(), from(RelParent, RelOperator)),
			RelAuditor:                  union(direct(), from(RelParent, RelAuditor)),
			RelCanOpenSession:           union(computed(RelOperator), computed(RelOwner)),
			RelHasActiveGrant:           directCond(ConditionValidWindow),
			RelCanOpenPrivilegedSession: intersection(computed(RelCanOpenSession), computed(RelHasActiveGrant)),
		},
		TypeAssetGroup: {
			RelParent:   direct(),
			RelOperator: union(direct(), from(RelParent, RelOperator)),
			RelAuditor:  union(direct(), from(RelParent, RelAuditor)),
		},
		TypeGroup: {
			RelMember: direct(),
		},
	},
}

// evalCondition reports whether a conditioned direct tuple currently applies. An
// unconditional relation ("") always applies; the valid_window condition applies
// only when the tuple carries a window AND the decision instant lies within it; a
// conditioned tuple with no window, or an unknown condition, FAILS CLOSED (grants
// nothing) — INV-6.
func evalCondition(condition string, w *ValidityWindow, ctx CheckContext) bool {
	switch condition {
	case "":
		return true
	case ConditionValidWindow:
		if w == nil {
			return false
		}
		return w.Contains(ctx.EvaluatedAt)
	default:
		return false
	}
}
