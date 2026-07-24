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
	"time"

	"github.com/google/uuid"
)

// loadInto applies a projected write into the in-memory graph, honoring a
// conditioned tuple's window — mirrors what the store/reader do end-to-end (T-010).
func loadInto(g *MemoryGraph, u TupleUpdate) {
	if u.Op != TupleWrite {
		return
	}
	if u.Condition != nil {
		g.AddConditioned(u.Tuple.Object, u.Tuple.Relation, u.Tuple.User, u.Condition.Window)
		return
	}
	g.Add(u.Tuple.Object, u.Tuple.Relation, u.Tuple.User)
}

func TestProjectGroupMembership(t *testing.T) {
	org, grp, mem := uuid.New(), uuid.New(), uuid.New()
	w := ProjectGroupMembership(org, grp, mem, true)
	if w.Op != TupleWrite || w.Tuple.Relation != RelMember {
		t.Fatalf("projeção de member inesperada: %+v", w)
	}
	if err := ValidateTuple(w.Tuple); err != nil {
		t.Fatalf("tupla de member deveria ser válida: %v", err)
	}
	if d := ProjectGroupMembership(org, grp, mem, false); d.Op != TupleDelete {
		t.Fatalf("saída do grupo deveria projetar delete, veio %s", d.Op)
	}
}

func TestProjectRoleAssignment(t *testing.T) {
	org := uuid.New()
	assetRef := Qualify(org, TypeAsset, uuid.New().String())
	groupUserset := QualifyUserset(org, TypeGroup, uuid.New().String(), RelMember)

	u, err := ProjectRoleAssignment(assetRef, RelOperator, groupUserset, true)
	if err != nil {
		t.Fatalf("operator para group#member deveria projetar: %v", err)
	}
	if u.Op != TupleWrite || u.Tuple.Relation != RelOperator {
		t.Fatalf("projeção de operator inesperada: %+v", u)
	}

	// Relação não atribuível é rejeitada.
	if _, err := ProjectRoleAssignment(assetRef, RelCanOpenSession, groupUserset, true); !errors.Is(err, ErrInvalidProjection) {
		t.Fatalf("can_open_session não é atribuível diretamente")
	}
	// Cross-tenant é rejeitado.
	other := Qualify(uuid.New(), TypeGroup, uuid.New().String())
	if _, err := ProjectRoleAssignment(assetRef, RelOperator, other, true); !errors.Is(err, ErrInvalidProjection) {
		t.Fatalf("atribuição cross-tenant deveria ser rejeitada")
	}
}

func TestProjectGrantActiveCarriesWindow(t *testing.T) {
	org, mem := uuid.New(), uuid.New()
	assetRef := Qualify(org, TypeAsset, uuid.New().String())
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC)
	grant := PrivilegedGrant{
		ID: uuid.New(), OrganizationID: org, SubjectMembershipID: mem,
		Status: GrantActive, NotBefore: nb, ExpiresAt: exp,
	}
	u, err := ProjectGrant(grant, assetRef)
	if err != nil {
		t.Fatalf("ProjectGrant: %v", err)
	}
	if u.Op != TupleWrite || u.Tuple.Relation != RelHasActiveGrant {
		t.Fatalf("concessão ativa deveria projetar write de has_active_grant: %+v", u)
	}
	if u.Condition == nil || u.Condition.Name != ConditionValidWindow ||
		!u.Condition.Window.NotBefore.Equal(nb) || !u.Condition.Window.ExpiresAt.Equal(exp) {
		t.Fatalf("a janela da concessão deveria acompanhar a tupla: %+v", u.Condition)
	}

	// Revogada -> delete.
	grant.Status = GrantRevoked
	if d, _ := ProjectGrant(grant, assetRef); d.Op != TupleDelete {
		t.Fatalf("concessão revogada deveria projetar delete, veio %s", d.Op)
	}
}

// Ponta-a-ponta da projeção: operador via grupo + concessão vigente abrem a sessão
// privilegiada; a MESMA concessão fora da janela nega (expira no grafo, RFC §3).
func TestProjectionResolvesPrivilegedAndExpires(t *testing.T) {
	org := uuid.New()
	assetID := uuid.New()
	assetRef := Qualify(org, TypeAsset, assetID.String())
	groupID := uuid.New()
	groupUserset := QualifyUserset(org, TypeGroup, groupID.String(), RelMember)
	mem := uuid.New()
	memRef := Qualify(org, TypeMembership, mem.String())

	g := NewMemoryGraph()
	// operator do grupo sobre o ativo
	roleTuple, _ := ProjectRoleAssignment(assetRef, RelOperator, groupUserset, true)
	loadInto(g, roleTuple)
	// membership é membro do grupo
	loadInto(g, ProjectGroupMembership(org, groupID, mem, true))
	// concessão vigente (janela cobre 12h)
	grant := PrivilegedGrant{
		ID: uuid.New(), OrganizationID: org, SubjectMembershipID: mem, Status: GrantActive,
		NotBefore: time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
	}
	grantTuple, _ := ProjectGrant(grant, assetRef)
	loadInto(g, grantTuple)

	within := CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}
	dec, err := Evaluate(g, assetRef, RelCanOpenPrivilegedSession, memRef, within)
	if err != nil || !dec.Allowed {
		t.Fatalf("dentro da janela deveria permitir a privilegiada: allowed=%v err=%v", dec.Allowed, err)
	}

	after := CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 13, 0, 1, 0, time.UTC)}
	dec2, err := Evaluate(g, assetRef, RelCanOpenPrivilegedSession, memRef, after)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if dec2.Allowed {
		t.Fatalf("fora da janela a privilegiada deveria ser negada (concessão expira no grafo)")
	}
	// A sessão NÃO privilegiada (só operator) segue permitida.
	if dec3, _ := Evaluate(g, assetRef, RelCanOpenSession, memRef, after); !dec3.Allowed {
		t.Fatalf("a sessão comum por operator deveria seguir permitida fora da janela da concessão")
	}
}
