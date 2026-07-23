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

func TestQualifyAndRefOrg(t *testing.T) {
	org := uuid.New()
	ref := Qualify(org, TypeAsset, "db-prod-03")
	if ref != "org:"+org.String()+"/asset:db-prod-03" {
		t.Fatalf("qualificação inesperada: %s", ref)
	}
	got, ok := RefOrg(ref)
	if !ok || got != org.String() {
		t.Fatalf("RefOrg deveria extrair %s, veio %q ok=%v", org, got, ok)
	}
	if !Qualified(ref) {
		t.Fatalf("ref deveria ser reconhecido como qualificado")
	}
	// Userset também expõe a org.
	us := QualifyUserset(org, TypeGroup, "dba", RelMember)
	if o, ok := RefOrg(us); !ok || o != org.String() {
		t.Fatalf("RefOrg do userset deveria extrair a org, veio %q ok=%v", o, ok)
	}
}

func TestRefOrgUnqualified(t *testing.T) {
	for _, ref := range []string{"asset:a1", "membership:m1", "org:/asset:a1", "", "asset"} {
		if _, ok := RefOrg(ref); ok {
			t.Fatalf("ref %q não deveria ser qualificado", ref)
		}
	}
}

func TestSameTenant(t *testing.T) {
	o1, o2 := uuid.New(), uuid.New()
	a := Qualify(o1, TypeMembership, "m1")
	b := Qualify(o1, TypeAsset, "x")
	c := Qualify(o2, TypeAsset, "x")
	if !SameTenant(a, b) {
		t.Fatalf("mesma org deveria casar")
	}
	if SameTenant(a, c) {
		t.Fatalf("orgs distintas não deveriam casar")
	}
	if SameTenant("membership:m1", b) {
		t.Fatalf("ref não qualificado nunca é mesmo tenant")
	}
}

// spec "Tentativa de relação cruzada": tupla relacionando tenants distintos é rejeitada.
func TestValidateTupleCrossTenantRejected(t *testing.T) {
	o1, o2 := uuid.New(), uuid.New()
	cross := RelationTuple{
		User:     Qualify(o1, TypeMembership, "m1"),
		Relation: RelOperator,
		Object:   Qualify(o2, TypeAsset, "a1"),
	}
	if err := ValidateTuple(cross); !errors.Is(err, ErrCrossTenantRelation) {
		t.Fatalf("tupla cruzada deveria ser ErrCrossTenantRelation, veio %v", err)
	}
}

func TestValidateTupleUnqualifiedRejected(t *testing.T) {
	o1 := uuid.New()
	bad := RelationTuple{
		User:     "membership:m1", // sem qualificação
		Relation: RelOperator,
		Object:   Qualify(o1, TypeAsset, "a1"),
	}
	if err := ValidateTuple(bad); !errors.Is(err, ErrUnqualifiedRef) {
		t.Fatalf("tupla não qualificada deveria ser ErrUnqualifiedRef, veio %v", err)
	}
}

func TestValidateTupleSameTenantAccepted(t *testing.T) {
	o1 := uuid.New()
	ok := RelationTuple{
		User:     QualifyUserset(o1, TypeGroup, "dba", RelMember),
		Relation: RelOperator,
		Object:   Qualify(o1, TypeAsset, "a1"),
	}
	if err := ValidateTuple(ok); err != nil {
		t.Fatalf("tupla intra-tenant (userset) deveria passar, veio %v", err)
	}
}

// spec "Consulta cruzada": check de membership do tenant A sobre ativo do tenant B é negado.
func TestGuardSameTenantCheck(t *testing.T) {
	o1, o2 := uuid.New(), uuid.New()
	if err := GuardSameTenant(Qualify(o1, TypeMembership, "m1"), Qualify(o1, TypeAsset, "a1")); err != nil {
		t.Fatalf("consulta intra-tenant deveria passar, veio %v", err)
	}
	if err := GuardSameTenant(Qualify(o1, TypeMembership, "m1"), Qualify(o2, TypeAsset, "a1")); !errors.Is(err, ErrCrossTenantRelation) {
		t.Fatalf("consulta cruzada deveria ser negada, veio %v", err)
	}
	if err := GuardSameTenant("membership:m1", Qualify(o1, TypeAsset, "a1")); !errors.Is(err, ErrUnqualifiedRef) {
		t.Fatalf("consulta com ref não qualificado deveria ser ErrUnqualifiedRef, veio %v", err)
	}
}
