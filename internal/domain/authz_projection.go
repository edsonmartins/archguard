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

	"github.com/google/uuid"
)

// This file derives authorization tuple deltas from domain facts — the projection
// of the source of truth into the graph (RFC-0004 §4). These are PURE functions
// (INV-3); wiring them to persistence and enqueuing on mutation lands with asset
// persistence (M4). Everything they produce is intra-tenant and tenant-qualified
// (T-003), so the outbox accepts it (INV-5).

// ErrInvalidProjection is returned when a projection input is malformed (e.g. an
// operator relation that is not operator/auditor).
var ErrInvalidProjection = errors.New("authz_projection: entrada de projeção inválida")

// assignableRelations are the relations a role assignment may grant on an asset or
// asset_group. can_open_session / can_open_privileged_session are DERIVED, never
// assigned directly; has_active_grant is projected from grants, not assignments.
func assignableRelation(relation string) bool {
	return relation == RelOperator || relation == RelAuditor
}

// ProjectGroupMembership derives the tuple that places a membership in a group
// (group `member`), through which group-scoped operator/auditor reaches the
// person. present=true → write; present=false → delete (the person left the
// group). Both refs are qualified by the group's organization.
func ProjectGroupMembership(orgID, groupID, membershipID uuid.UUID, present bool) TupleUpdate {
	return TupleUpdate{
		Op: opFor(present),
		Tuple: RelationTuple{
			User:     Qualify(orgID, TypeMembership, membershipID.String()),
			Relation: RelMember,
			Object:   Qualify(orgID, TypeGroup, groupID.String()),
		},
	}
}

// ProjectRoleAssignment derives the tuple granting a subject operator/auditor on an
// object (an asset or asset_group). subjectRef is a membership ref or a group
// userset ("...group:<id>#member"); objectRef is a tenant-qualified asset or
// asset_group. The relation must be assignable and the two refs must share a
// tenant (INV-5). present=false → delete.
func ProjectRoleAssignment(objectRef, relation, subjectRef string, present bool) (TupleUpdate, error) {
	if !assignableRelation(relation) {
		return TupleUpdate{}, fmt.Errorf("%w: relação %q não é atribuível", ErrInvalidProjection, relation)
	}
	tuple := RelationTuple{User: subjectRef, Relation: relation, Object: objectRef}
	if err := ValidateTuple(tuple); err != nil {
		return TupleUpdate{}, fmt.Errorf("%w: %v", ErrInvalidProjection, err)
	}
	return TupleUpdate{Op: opFor(present), Tuple: tuple}, nil
}

// ProjectGrant derives the has_active_grant conditioned tuple for a privileged
// grant on an asset (assetRef, tenant-qualified). An ACTIVE grant projects a
// WRITE carrying the grant's [NotBefore, ExpiresAt) window as the valid_window
// condition — so the concession expires IN THE GRAPH (RFC-0004 §3), and a token
// minted before expiry no longer authorizes past the window. Any other status
// (revoked, expired, rejected, still awaiting) projects a DELETE, removing the
// tuple; expiry itself needs no delete because the condition already denies.
func ProjectGrant(grant PrivilegedGrant, assetRef string) (TupleUpdate, error) {
	subject := Qualify(grant.OrganizationID, TypeMembership, grant.SubjectMembershipID.String())
	tuple := RelationTuple{User: subject, Relation: RelHasActiveGrant, Object: assetRef}
	if err := ValidateTuple(tuple); err != nil {
		return TupleUpdate{}, fmt.Errorf("%w: %v", ErrInvalidProjection, err)
	}
	if grant.Status != GrantActive {
		return TupleUpdate{Op: TupleDelete, Tuple: tuple}, nil
	}
	return TupleUpdate{
		Op:    TupleWrite,
		Tuple: tuple,
		Condition: &TupleCondition{
			Name:   ConditionValidWindow,
			Window: ValidityWindow{NotBefore: grant.NotBefore, ExpiresAt: grant.ExpiresAt},
		},
	}, nil
}

// ProjectAsset derives an asset's structural tuples (parent, owner) as writes —
// the projection form of Asset.Tuples(). present=false projects deletes (the asset
// was removed or re-parented).
func ProjectAsset(a Asset, present bool) []TupleUpdate {
	structural := a.Tuples()
	out := make([]TupleUpdate, 0, len(structural))
	for _, tup := range structural {
		out = append(out, TupleUpdate{Op: opFor(present), Tuple: tup})
	}
	return out
}

func opFor(present bool) TupleOp {
	if present {
		return TupleWrite
	}
	return TupleDelete
}
