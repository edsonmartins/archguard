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

func TestNewAssetValidates(t *testing.T) {
	org := uuid.New()
	if _, err := NewAsset(uuid.Nil, "host", "db-prod-03", "", nil, nil); !errors.Is(err, ErrInvalidAsset) {
		t.Fatalf("sem organização deveria falhar")
	}
	if _, err := NewAsset(org, "", "db-prod-03", "", nil, nil); !errors.Is(err, ErrInvalidAsset) {
		t.Fatalf("sem tipo deveria falhar")
	}
	if _, err := NewAsset(org, "host", "", "", nil, nil); !errors.Is(err, ErrInvalidAsset) {
		t.Fatalf("sem nome deveria falhar")
	}
	a, err := NewAsset(org, "host", "db-prod-03", "warpgate:host-42", nil, nil)
	if err != nil {
		t.Fatalf("asset válido: %v", err)
	}
	if a.Ref() != "org:"+org.String()+"/asset:"+a.ID.String() {
		t.Fatalf("ref inesperado: %s", a.Ref())
	}
}

// As tuplas projetadas (parent + owner) são intra-tenant e passam ValidateTuple.
func TestAssetTuplesProjection(t *testing.T) {
	org := uuid.New()
	grp, err := NewAssetGroup(org, "cluster-oracle-prod", nil)
	if err != nil {
		t.Fatalf("grupo: %v", err)
	}
	owner := uuid.New()
	a, err := NewAsset(org, "host", "db-prod-03", "", &grp.ID, &owner)
	if err != nil {
		t.Fatalf("asset: %v", err)
	}
	tuples := a.Tuples()
	if len(tuples) != 2 {
		t.Fatalf("esperava 2 tuplas (parent+owner), veio %d: %+v", len(tuples), tuples)
	}
	for _, tup := range tuples {
		if err := ValidateTuple(tup); err != nil {
			t.Fatalf("tupla projetada deveria ser válida intra-tenant: %+v err=%v", tup, err)
		}
		if tup.Object != a.Ref() {
			t.Fatalf("objeto da tupla deveria ser o asset, veio %s", tup.Object)
		}
	}
}

// Um asset em um grupo com pai: a herança resolve pelo modelo (integração com T-002).
func TestAssetInheritanceResolvesThroughProjectedTuples(t *testing.T) {
	org := uuid.New()
	cluster, _ := NewAssetGroup(org, "cluster", nil)
	sub, _ := NewAssetGroup(org, "sub", &cluster.ID)
	a, _ := NewAsset(org, "host", "db-prod-03", "", &sub.ID, nil)
	oper := uuid.New()
	operRef := Qualify(org, TypeMembership, oper.String())

	g := NewMemoryGraph()
	for _, tup := range append(append(a.Tuples(), sub.Tuples()...), cluster.Tuples()...) {
		g.Add(tup.Object, tup.Relation, tup.User)
	}
	// Operador no cluster (topo da hierarquia): object=cluster, subject=operRef.
	g.Add(cluster.Ref(), RelOperator, operRef)

	dec, err := Evaluate(g, a.Ref(), RelCanOpenSession, operRef,
		CheckContext{ACR: "L2"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("operador do cluster deveria abrir sessão no host filho por herança")
	}
}

func TestValidateAssetGroupHierarchyTreeOK(t *testing.T) {
	org := uuid.New()
	root, _ := NewAssetGroup(org, "root", nil)
	child, _ := NewAssetGroup(org, "child", &root.ID)
	groups := map[uuid.UUID]AssetGroup{root.ID: root, child.ID: child}
	if err := ValidateAssetGroupHierarchy(groups); err != nil {
		t.Fatalf("árvore válida não deveria acusar: %v", err)
	}
}

func TestValidateAssetGroupHierarchyCycle(t *testing.T) {
	org := uuid.New()
	a, _ := NewAssetGroup(org, "a", nil)
	b, _ := NewAssetGroup(org, "b", nil)
	// Cria ciclo a→b→a mutando os ponteiros.
	a.ParentGroupID = &b.ID
	b.ParentGroupID = &a.ID
	groups := map[uuid.UUID]AssetGroup{a.ID: a, b.ID: b}
	if err := ValidateAssetGroupHierarchy(groups); !errors.Is(err, ErrAssetGroupCycle) {
		t.Fatalf("ciclo deveria ser detectado, veio %v", err)
	}
}

func TestValidateAssetGroupHierarchyCrossTenantParent(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	root, _ := NewAssetGroup(orgB, "root-b", nil)
	child, _ := NewAssetGroup(orgA, "child-a", &root.ID) // pai em outro tenant
	groups := map[uuid.UUID]AssetGroup{root.ID: root, child.ID: child}
	if err := ValidateAssetGroupHierarchy(groups); !errors.Is(err, ErrCrossTenantRelation) {
		t.Fatalf("pai em outro tenant deveria ser rejeitado, veio %v", err)
	}
}

func TestValidateAssetGroupHierarchyDanglingParent(t *testing.T) {
	org := uuid.New()
	child, _ := NewAssetGroup(org, "child", nil)
	ghost := uuid.New()
	child.ParentGroupID = &ghost
	groups := map[uuid.UUID]AssetGroup{child.ID: child}
	if err := ValidateAssetGroupHierarchy(groups); !errors.Is(err, ErrInvalidAssetGroup) {
		t.Fatalf("pai inexistente deveria ser rejeitado, veio %v", err)
	}
}
