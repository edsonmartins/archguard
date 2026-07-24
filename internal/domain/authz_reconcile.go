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

import "sort"

// Reconciliation compares the derived state expected from the source of truth
// with the tuples actually in the PDP store, and classifies each divergence by
// its EFFECT ON ACCESS (RFC-0004 §6 / spec "Reconciliação com política
// assimétrica"). The asymmetry is the safety property: a divergence that
// RESTRICTS access is corrected automatically, while a divergence that would
// AMPLIFY access is never applied automatically — automatic amplification is a
// silent-escalation vector — and is raised for human review.

// ReconcileDiff classifies the difference between expected (from source) and
// actual (in the store) tuple sets:
//
//   - Extra: tuples PRESENT in the store but NOT expected. They grant access the
//     source does not — removing them RESTRICTS access, so they are the automatic
//     correction set.
//   - Missing: tuples EXPECTED but ABSENT from the store. Adding them would AMPLIFY
//     access, so they are NEVER auto-applied — they are alerted for human review.
//
// Both slices are returned in a deterministic (sorted) order. The comparison is by
// (user, relation, object); conditions/windows are not part of tuple identity
// (a conditioned tuple diverging only in its window reconciles as the same tuple).
//
// SAFETY: this is only sound when `expected` is AUTHORITATIVE and COMPLETE. Run it
// with a partial source projection and every real tuple looks Extra and would be
// removed — callers must not reconcile against an incomplete expected set.
func ReconcileDiff(expected, actual []RelationTuple) (extra, missing []RelationTuple) {
	expectedSet := make(map[string]RelationTuple, len(expected))
	for _, tup := range expected {
		expectedSet[tupleKey(tup)] = tup
	}
	actualSet := make(map[string]RelationTuple, len(actual))
	for _, tup := range actual {
		actualSet[tupleKey(tup)] = tup
	}
	for k, tup := range actualSet {
		if _, ok := expectedSet[k]; !ok {
			extra = append(extra, tup)
		}
	}
	for k, tup := range expectedSet {
		if _, ok := actualSet[k]; !ok {
			missing = append(missing, tup)
		}
	}
	sortTuples(extra)
	sortTuples(missing)
	return extra, missing
}

// tupleKey is the identity of a tuple for set comparison.
func tupleKey(t RelationTuple) string {
	return t.User + "\x00" + t.Relation + "\x00" + t.Object
}

func sortTuples(ts []RelationTuple) {
	sort.Slice(ts, func(i, j int) bool { return tupleKey(ts[i]) < tupleKey(ts[j]) })
}

// ReconcileReport is the outcome of one reconciliation pass over a tenant.
type ReconcileReport struct {
	// Removed are the extra tuples that were automatically removed (restrictive
	// correction). Each removal is auditable.
	Removed []RelationTuple
	// MissingAlerted are the expected-but-absent tuples raised for human review;
	// they were NOT applied (amplifying correction is never automatic).
	MissingAlerted []RelationTuple
}

// Diverged reports whether the pass found any divergence at all.
func (r ReconcileReport) Diverged() bool {
	return len(r.Removed) > 0 || len(r.MissingAlerted) > 0
}
