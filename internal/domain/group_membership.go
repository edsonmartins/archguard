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

// GroupMembership is the source-of-truth record that a membership belongs to an access
// group (pacote 007 M4, T-029 D1). It projects the `member` tuple, through which a group
// that holds operator/auditor over an asset reaches the person (membership → member →
// group → operator → asset). The access group is an opaque id (no group entity in this
// slice).
type GroupMembership struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	GroupID        uuid.UUID
	MembershipID   uuid.UUID
}

// ErrInvalidGroupMembership is returned when the binding is malformed.
var ErrInvalidGroupMembership = errors.New("group_membership: dados obrigatórios ausentes")

// NewGroupMembership builds a validated binding (UUIDv7 id).
func NewGroupMembership(orgID, groupID, membershipID uuid.UUID) (GroupMembership, error) {
	if orgID == uuid.Nil || groupID == uuid.Nil || membershipID == uuid.Nil {
		return GroupMembership{}, fmt.Errorf("%w: organização, grupo e membership", ErrInvalidGroupMembership)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return GroupMembership{}, fmt.Errorf("group_membership: geração de UUIDv7 falhou: %w", err)
	}
	return GroupMembership{ID: id, OrganizationID: orgID, GroupID: groupID, MembershipID: membershipID}, nil
}

// Tuple projects the binding to its `member` TupleUpdate (write when present, delete
// otherwise), reusing the shared ProjectGroupMembership derivation.
func (g GroupMembership) Tuple(present bool) TupleUpdate {
	return ProjectGroupMembership(g.OrganizationID, g.GroupID, g.MembershipID, present)
}
