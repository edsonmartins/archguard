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

// Package rolemigration re-points a legacy role's denormalized member list onto
// explicit role_assignment rows keyed by MEMBERSHIP (RFC-0002 §2.4, R2, pacote
// 002 T-006). It is the MECHANISM: bulk execution over all roles is T-019, once
// identities and memberships exist.
//
// It depends on a MembershipResolver to map each legacy "org/user" identifier to
// a membership; the real resolver (over the identity/membership stores) is wired
// at execution. Users with no membership are skipped and reported, never guessed.
package rolemigration

import (
	"context"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ResolvedMembership is the membership a legacy user maps to, with its tenant.
type ResolvedMembership struct {
	MembershipID   uuid.UUID
	OrganizationID uuid.UUID
}

// MembershipResolver maps a legacy "org/user" identifier to a membership. found
// is false when the user has no membership (e.g. the identity was never created,
// or is not a member of that organization) — the assignment is then skipped.
type MembershipResolver interface {
	Resolve(ctx context.Context, orgUser string) (ResolvedMembership, bool, error)
}

// Result is the outcome of re-pointing one role's members.
type Result struct {
	// Assignments are the role↔membership bindings to persist.
	Assignments []domain.RoleAssignment
	// Unresolved are the legacy user identifiers that had no membership and were
	// skipped. Surfaced (never silently dropped) so execution can report them.
	Unresolved []string
}

// Migrate builds role_assignment rows for roleID from the legacy user list,
// resolving each user to a membership. A user that resolves to no membership is
// recorded in Unresolved and skipped. Duplicate memberships in the input yield a
// single assignment.
func Migrate(ctx context.Context, roleID uuid.UUID, users []string, r MembershipResolver) (Result, error) {
	if roleID == uuid.Nil {
		return Result{}, fmt.Errorf("rolemigration: roleID nulo")
	}
	var res Result
	seen := map[uuid.UUID]bool{}
	for _, u := range users {
		if u == "" {
			continue
		}
		rm, found, err := r.Resolve(ctx, u)
		if err != nil {
			return Result{}, fmt.Errorf("rolemigration: resolução de %q falhou: %w", u, err)
		}
		if !found {
			res.Unresolved = append(res.Unresolved, u)
			continue
		}
		if seen[rm.MembershipID] {
			continue
		}
		seen[rm.MembershipID] = true
		ra, err := domain.NewRoleAssignment(rm.OrganizationID, roleID, rm.MembershipID)
		if err != nil {
			return Result{}, fmt.Errorf("rolemigration: vínculo para %q: %w", u, err)
		}
		res.Assignments = append(res.Assignments, ra)
	}
	return res, nil
}
