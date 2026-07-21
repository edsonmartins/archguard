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

// ErrInvalidRoleAssignment is returned when constructing an assignment with any
// missing reference.
var ErrInvalidRoleAssignment = errors.New("role_assignment: referências obrigatórias ausentes")

// RoleAssignment binds a role to a MEMBERSHIP, never to the global identity
// (RFC-0002 R2): a role is always held in the context of one organization. This
// is what keeps an administrative role in organization A from following the
// person into organization B.
//
// The type encodes R2 structurally: it references MembershipID, and there is no
// IdentityID field to bind a role to. OrganizationID is the tenant of the
// membership (R1) and the key the RLS policy filters on (T-010).
type RoleAssignment struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	RoleID         uuid.UUID
	MembershipID   uuid.UUID
}

// NewRoleAssignment binds roleID to membershipID within organizationID. All
// three references are required; organizationID must be the membership's own
// tenant (the caller guarantees role and membership share it).
func NewRoleAssignment(organizationID, roleID, membershipID uuid.UUID) (RoleAssignment, error) {
	if organizationID == uuid.Nil || roleID == uuid.Nil || membershipID == uuid.Nil {
		return RoleAssignment{}, ErrInvalidRoleAssignment
	}
	id, err := uuid.NewV7()
	if err != nil {
		return RoleAssignment{}, fmt.Errorf("role_assignment: geração de UUIDv7 falhou: %w", err)
	}
	return RoleAssignment{
		ID:             id,
		OrganizationID: organizationID,
		RoleID:         roleID,
		MembershipID:   membershipID,
	}, nil
}
