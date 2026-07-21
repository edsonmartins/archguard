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

package rolemigration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// fakeResolver maps a fixed set of "org/user" identifiers to memberships.
type fakeResolver struct {
	m map[string]ResolvedMembership
}

func (f fakeResolver) Resolve(_ context.Context, orgUser string) (ResolvedMembership, bool, error) {
	rm, ok := f.m[orgUser]
	return rm, ok, nil
}

func newID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestMigrateResolvesUsersToMemberships(t *testing.T) {
	roleID := newID(t)
	org := newID(t)
	memAlice, memBob := newID(t), newID(t)
	r := fakeResolver{m: map[string]ResolvedMembership{
		"acme/alice": {MembershipID: memAlice, OrganizationID: org},
		"acme/bob":   {MembershipID: memBob, OrganizationID: org},
	}}

	res, err := Migrate(context.Background(), roleID, []string{"acme/alice", "acme/bob"}, r)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(res.Assignments) != 2 {
		t.Fatalf("esperava 2 vínculos, veio %d", len(res.Assignments))
	}
	if len(res.Unresolved) != 0 {
		t.Errorf("não deveria haver não-resolvidos: %v", res.Unresolved)
	}
	for _, ra := range res.Assignments {
		if ra.RoleID != roleID || ra.OrganizationID != org {
			t.Error("vínculo com role/org errados")
		}
		// R2: o vínculo referencia membership, nunca identity.
		if ra.MembershipID != memAlice && ra.MembershipID != memBob {
			t.Errorf("membership inesperado: %v", ra.MembershipID)
		}
	}
}

func TestMigrateReportsUnresolved(t *testing.T) {
	roleID := newID(t)
	org := newID(t)
	r := fakeResolver{m: map[string]ResolvedMembership{
		"acme/alice": {MembershipID: newID(t), OrganizationID: org},
	}}
	res, err := Migrate(context.Background(), roleID, []string{"acme/alice", "acme/ghost", ""}, r)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(res.Assignments) != 1 {
		t.Fatalf("esperava 1 vínculo, veio %d", len(res.Assignments))
	}
	if len(res.Unresolved) != 1 || res.Unresolved[0] != "acme/ghost" {
		t.Errorf("não-resolvidos = %v, quer [acme/ghost]", res.Unresolved)
	}
}

func TestMigrateDedupesByMembership(t *testing.T) {
	roleID := newID(t)
	org := newID(t)
	mem := newID(t)
	// Dois identificadores legados resolvem para o mesmo membership (ex.: alias).
	r := fakeResolver{m: map[string]ResolvedMembership{
		"acme/alice":   {MembershipID: mem, OrganizationID: org},
		"acme/alice.2": {MembershipID: mem, OrganizationID: org},
	}}
	res, err := Migrate(context.Background(), roleID, []string{"acme/alice", "acme/alice.2"}, r)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(res.Assignments) != 1 {
		t.Errorf("membership repetido deveria gerar 1 vínculo, veio %d", len(res.Assignments))
	}
}

func TestMigrateRejectsNilRole(t *testing.T) {
	if _, err := Migrate(context.Background(), uuid.Nil, []string{"a/b"}, fakeResolver{}); err == nil {
		t.Error("roleID nulo deveria ser rejeitado")
	}
}
