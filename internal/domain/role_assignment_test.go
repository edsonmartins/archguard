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
	"testing"

	"github.com/google/uuid"
)

func TestNewRoleAssignment(t *testing.T) {
	org, role, mem := credID(t), credID(t), credID(t)
	ra, err := NewRoleAssignment(org, role, mem)
	if err != nil {
		t.Fatalf("NewRoleAssignment: %v", err)
	}
	if ra.ID.Version() != 7 {
		t.Errorf("id deveria ser UUIDv7, veio %d", ra.ID.Version())
	}
	if ra.OrganizationID != org || ra.RoleID != role || ra.MembershipID != mem {
		t.Error("referências não preservadas")
	}
}

func TestNewRoleAssignmentRejectsNilRefs(t *testing.T) {
	v := credID(t)
	cases := []struct {
		name           string
		org, role, mem uuid.UUID
	}{
		{"org nula", uuid.Nil, v, v},
		{"role nula", v, uuid.Nil, v},
		{"membership nula", v, v, uuid.Nil},
	}
	for _, c := range cases {
		if _, err := NewRoleAssignment(c.org, c.role, c.mem); !errors.Is(err, ErrInvalidRoleAssignment) {
			t.Errorf("%s: erro = %v, quer ErrInvalidRoleAssignment", c.name, err)
		}
	}
}
