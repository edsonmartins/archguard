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

// TestGroupMembershipTuple: o vínculo projeta a tupla `member` (membership → member → group).
func TestGroupMembershipTuple(t *testing.T) {
	org, grp, mem := uuid.New(), uuid.New(), uuid.New()
	g, err := NewGroupMembership(org, grp, mem)
	if err != nil {
		t.Fatalf("NewGroupMembership: %v", err)
	}
	upd := g.Tuple(true)
	if upd.Op != TupleWrite || upd.Tuple.Relation != RelMember {
		t.Errorf("tuple = %+v", upd)
	}
	if upd.Tuple.User != Qualify(org, TypeMembership, mem.String()) || upd.Tuple.Object != Qualify(org, TypeGroup, grp.String()) {
		t.Errorf("refs errados: %+v", upd.Tuple)
	}
	if _, err := NewGroupMembership(org, uuid.Nil, mem); !errors.Is(err, ErrInvalidGroupMembership) {
		t.Error("grupo nulo deveria falhar")
	}
}

// TestAssetAccessGroupSubject: um assignment com sujeito GROUP projeta a relação sobre o
// userset `group:<id>#member` — por onde os membros herdam o acesso (D1).
func TestAssetAccessGroupSubject(t *testing.T) {
	org, grp, asset := uuid.New(), uuid.New(), uuid.New()
	a, err := NewAssetAccessAssignment(org, TypeGroup, grp, RelOperator, TypeAsset, asset)
	if err != nil {
		t.Fatalf("assignment group→operator deveria valer: %v", err)
	}
	want := Qualify(org, TypeGroup, grp.String()) + "#member"
	if a.SubjectRef() != want {
		t.Errorf("subjectRef = %q, esperado %q", a.SubjectRef(), want)
	}
	upd, err := a.Tuple(true)
	if err != nil || upd.Tuple.User != want || upd.Tuple.Relation != RelOperator {
		t.Errorf("tuple = %+v err=%v", upd, err)
	}
	// sujeito inválido.
	if _, err := NewAssetAccessAssignment(org, TypeAsset, grp, RelOperator, TypeAsset, asset); !errors.Is(err, ErrInvalidAssetAccess) {
		t.Error("sujeito asset não deveria valer")
	}
}
