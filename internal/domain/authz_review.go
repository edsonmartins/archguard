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

import "strings"

// Access review is the reverse query behind "quem tem acesso efetivo a este
// ativo, e por quê?" (RFC-0004 §2 / spec "Campanha de revisão"). It reuses the
// same resolver that decides access, so a review can never disagree with a live
// decision — the point of making authorization explainable (ADR-0005).

// AccessOrigin classifies WHY a membership has access to an asset.
type AccessOrigin string

const (
	// OriginDirect: access granted on the asset itself (operator/owner assigned
	// directly, or through a group assigned directly on the asset).
	OriginDirect AccessOrigin = "direto"
	// OriginInherited: access inherited from an ancestor asset_group (operator
	// from parent).
	OriginInherited AccessOrigin = "herdado"
	// OriginGrant: the membership additionally holds an active privileged grant on
	// the asset (has_active_grant) — the origin that unlocks a privileged session.
	OriginGrant AccessOrigin = "concessão"
)

// AccessReviewEntry is one membership's effective access to the reviewed asset and
// how it arises. Origins may hold more than one value (e.g. inherited operator AND
// an active grant).
type AccessReviewEntry struct {
	Subject       string // membership ref
	Origins       []AccessOrigin
	Justification string // the resolver's path for the base (session) access
}

// classifyBaseOrigin maps the resolver justification of can_open_session access to
// direct vs inherited. The mapping is pinned by tests against the resolver's own
// output, so a change to the justification format is caught. Inheritance is marked
// by the tuple-to-userset " from " hop.
func classifyBaseOrigin(justification string) AccessOrigin {
	if strings.Contains(justification, " from ") {
		return OriginInherited
	}
	return OriginDirect
}

// ReviewAssetAccess evaluates, among candidate membership refs, those with
// effective can_open_session on assetRef, classifying each one's origin. A
// membership with no access is omitted. It fails closed: any resolver error aborts
// the review (a partial review must never read as complete). candidates are
// typically every membership ref present in the tenant's graph.
func ReviewAssetAccess(reader GraphReader, assetRef string, candidates []string, ctx CheckContext) ([]AccessReviewEntry, error) {
	var out []AccessReviewEntry
	for _, subject := range candidates {
		base, err := Evaluate(reader, assetRef, RelCanOpenSession, subject, ctx)
		if err != nil {
			return nil, err
		}
		if !base.Allowed {
			continue
		}
		origins := []AccessOrigin{classifyBaseOrigin(base.Justification)}

		grant, err := Evaluate(reader, assetRef, RelHasActiveGrant, subject, ctx)
		if err != nil {
			return nil, err
		}
		if grant.Allowed {
			origins = append(origins, OriginGrant)
		}
		out = append(out, AccessReviewEntry{Subject: subject, Origins: origins, Justification: base.Justification})
	}
	return out, nil
}

// HasOrigin reports whether the entry includes a given origin.
func (e AccessReviewEntry) HasOrigin(o AccessOrigin) bool {
	for _, got := range e.Origins {
		if got == o {
			return true
		}
	}
	return false
}
