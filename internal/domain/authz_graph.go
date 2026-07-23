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
	"strings"
)

// This file evaluates the authorization model (authz_model.go) against a set of
// relation tuples: the pure resolver behind the PDP's Check. It is the ArchGuard
// in-domain equivalent of an OpenFGA check, computed from the projected tuples.

// GraphSubject is a subject directly assigned to (object, relation): either a
// concrete ref ("membership:<id>") or a userset ("group:<id>#member"), with an
// optional validity window for conditioned relations.
type GraphSubject struct {
	// Ref is the subject reference. A userset ref contains '#': "<object>#<relation>".
	Ref string
	// Window, when non-nil, is the temporal window of a conditioned tuple (a grant).
	// nil for unconditional tuples.
	Window *ValidityWindow
}

// GraphReader is the read surface the resolver needs over the tuple set: the
// subjects directly assigned to (object, relation). Any store (in-memory,
// PostgreSQL projection) can implement it — keeping the resolver independent of
// where tuples live.
type GraphReader interface {
	DirectSubjects(object, relation string) []GraphSubject
}

// maxResolveDepth bounds recursion. A well-formed model resolves in a handful of
// hops; exceeding this signals a malformed model or a pathological chain, and the
// resolver FAILS CLOSED (errResolveDepth → the caller denies).
const maxResolveDepth = 128

// errResolveDepth is returned when resolution exceeds maxResolveDepth. It is an
// infrastructure-class error (the model is unusable), distinct from a computed
// denial, so the audit records `error` not `denied` (INV-6).
var errResolveDepth = errors.New("authz: profundidade de resolução excedida — negado (fail-closed)")

// Evaluate answers whether user holds relation on object under ctx, by resolving
// the model's rewrite tree over the reader's tuples. A computed refusal is
// (Decision{Allowed:false}, nil); a malformed-model failure is (_, error) and the
// caller denies. This is the pure core the in-domain PDP wraps (T-010).
func Evaluate(g GraphReader, object, relation, user string, ctx CheckContext) (Decision, error) {
	s := &resolveState{g: g, ctx: ctx, visiting: map[string]bool{}}
	ok, why, err := s.resolve(object, relation, user)
	if err != nil {
		return Decision{}, err
	}
	if !ok {
		return DenyDecision("sem relação"), nil
	}
	return Allow(why), nil
}

type resolveState struct {
	g        GraphReader
	ctx      CheckContext
	visiting map[string]bool // (object#relation) on the current stack — cycle guard
	depth    int
}

// resolve reports whether user holds relation on object, following the relation's
// rewrite. Unknown type/relation ⇒ not related (deny). A cycle contributes
// nothing (returns false). Depth overflow fails closed with an error.
func (s *resolveState) resolve(object, relation, user string) (bool, string, error) {
	if s.depth >= maxResolveDepth {
		return false, "", errResolveDepth
	}
	key := object + "#" + relation
	if s.visiting[key] {
		return false, "", nil
	}
	rels, ok := archGuardModel.relations[ObjectType(refType(object))]
	if !ok {
		return false, "", nil
	}
	rw, ok := rels[relation]
	if !ok {
		return false, "", nil
	}
	s.visiting[key] = true
	s.depth++
	ok, why, err := s.applyRewrite(object, relation, user, rw)
	s.depth--
	delete(s.visiting, key)
	return ok, why, err
}

// applyRewrite evaluates one rewrite node for (object, relation, user).
func (s *resolveState) applyRewrite(object, relation, user string, rw rewrite) (bool, string, error) {
	switch rw.kind {
	case rwThis:
		for _, sub := range s.g.DirectSubjects(object, relation) {
			if !evalCondition(rw.condition, sub.Window, s.ctx) {
				continue // window not satisfied ⇒ this tuple grants nothing now
			}
			if obj, rel, isUserset := splitUserset(sub.Ref); isUserset {
				ok, _, err := s.resolve(obj, rel, user)
				if err != nil {
					return false, "", err
				}
				if ok {
					return true, "via " + sub.Ref, nil
				}
				continue
			}
			if sub.Ref == user {
				return true, relation + " direto", nil
			}
		}
		return false, "", nil

	case rwComputed:
		ok, why, err := s.resolve(object, rw.relation, user)
		if err != nil || !ok {
			return false, "", err
		}
		return true, relation + " via " + why, nil

	case rwTTU:
		// Hop through the tupleset relation (e.g. parent) to each reached object and
		// evaluate the computed relation there — inheritance up the hierarchy.
		for _, sub := range s.g.DirectSubjects(object, rw.tupleset) {
			if _, _, isUserset := splitUserset(sub.Ref); isUserset {
				continue // tupleset targets are concrete objects, never usersets
			}
			ok, _, err := s.resolve(sub.Ref, rw.relation, user)
			if err != nil {
				return false, "", err
			}
			if ok {
				return true, rw.relation + " from " + rw.tupleset + " " + sub.Ref, nil
			}
		}
		return false, "", nil

	case rwUnion:
		for _, child := range rw.children {
			ok, why, err := s.applyRewrite(object, relation, user, child)
			if err != nil {
				return false, "", err
			}
			if ok {
				return true, why, nil
			}
		}
		return false, "", nil

	case rwIntersection:
		reasons := make([]string, 0, len(rw.children))
		for _, child := range rw.children {
			ok, why, err := s.applyRewrite(object, relation, user, child)
			if err != nil {
				return false, "", err
			}
			if !ok {
				return false, "", nil
			}
			reasons = append(reasons, why)
		}
		return true, relation + " = " + strings.Join(reasons, " ∧ "), nil

	default:
		// Unknown rewrite kind ⇒ fail closed.
		return false, "", nil
	}
}

// refType extracts the object type from a ref, tolerating both the plain form
// ("asset:a1") and the tenant-qualified form ("org:o1/asset:a1", T-003), and
// stripping any userset suffix ("group:g1#member" → "group").
func refType(ref string) string {
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndexByte(ref, '/'); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.IndexByte(ref, ':'); i >= 0 {
		return ref[:i]
	}
	return ref
}

// splitUserset splits a userset ref "<object>#<relation>" into its parts. isUserset
// is false for a concrete ref (no '#').
func splitUserset(ref string) (object, relation string, isUserset bool) {
	i := strings.IndexByte(ref, '#')
	if i < 0 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}

// MemoryGraph is an in-memory GraphReader over relation tuples — the store the
// resolver's tests use, and a ready projection target for a homolog PDP before
// the external engine exists. It is safe for single-threaded build-up then
// read-only evaluation (the usage pattern of a decision).
type MemoryGraph struct {
	subjects map[string][]GraphSubject // key: object + "\x00" + relation
}

// NewMemoryGraph builds an empty graph.
func NewMemoryGraph() *MemoryGraph {
	return &MemoryGraph{subjects: map[string][]GraphSubject{}}
}

func memKey(object, relation string) string { return object + "\x00" + relation }

// Add inserts an unconditional tuple (object, relation, subject). subject is a
// concrete ref or a userset ("group:g#member").
func (g *MemoryGraph) Add(object, relation, subject string) {
	k := memKey(object, relation)
	g.subjects[k] = append(g.subjects[k], GraphSubject{Ref: subject})
}

// AddConditioned inserts a tuple gated by a validity window (a grant): it applies
// only while the decision instant lies within [w.NotBefore, w.ExpiresAt).
func (g *MemoryGraph) AddConditioned(object, relation, subject string, w ValidityWindow) {
	k := memKey(object, relation)
	win := w
	g.subjects[k] = append(g.subjects[k], GraphSubject{Ref: subject, Window: &win})
}

// DirectSubjects returns the subjects directly assigned to (object, relation).
func (g *MemoryGraph) DirectSubjects(object, relation string) []GraphSubject {
	return g.subjects[memKey(object, relation)]
}

var _ GraphReader = (*MemoryGraph)(nil)
