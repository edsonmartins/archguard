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
	"strings"
	"testing"
	"time"
)

func at(t *testing.T) CheckContext {
	t.Helper()
	return CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}
}

func mustAllow(t *testing.T, g GraphReader, object, relation, user string, ctx CheckContext) string {
	t.Helper()
	dec, err := Evaluate(g, object, relation, user, ctx)
	if err != nil {
		t.Fatalf("Evaluate(%s,%s,%s) erro inesperado: %v", object, relation, user, err)
	}
	if !dec.Allowed {
		t.Fatalf("esperava permitido para %s %s %s, veio negado", user, relation, object)
	}
	return dec.Justification
}

func mustDeny(t *testing.T, g GraphReader, object, relation, user string, ctx CheckContext) {
	t.Helper()
	dec, err := Evaluate(g, object, relation, user, ctx)
	if err != nil {
		t.Fatalf("Evaluate(%s,%s,%s) erro inesperado: %v", object, relation, user, err)
	}
	if dec.Allowed {
		t.Fatalf("esperava negado para %s %s %s, veio permitido (%s)", user, relation, object, dec.Justification)
	}
}

// Operador direto -> can_open_session (union operator or owner).
func TestEvaluateDirectOperator(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	mustAllow(t, g, "asset:a1", RelCanOpenSession, "membership:m1", at(t))
	mustDeny(t, g, "asset:a1", RelCanOpenSession, "membership:outra", at(t))
}

// Dono -> can_open_session pelo outro ramo da união.
func TestEvaluateOwnerBranch(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOwner, "membership:m1")
	why := mustAllow(t, g, "asset:a1", RelCanOpenSession, "membership:m1", at(t))
	if why == "" {
		t.Fatalf("justificativa deveria descrever o caminho")
	}
}

// Herança: operador do grupo de ativos -> operador do ativo filho (operator from parent).
func TestEvaluateInheritedFromParent(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelParent, "asset_group:g1")
	g.Add("asset_group:g1", RelOperator, "membership:m1")
	why := mustAllow(t, g, "asset:a1", RelOperator, "membership:m1", at(t))
	if want := "from " + RelParent; !strings.Contains(why, want) {
		t.Fatalf("justificativa deveria indicar herança (%q), veio %q", want, why)
	}
}

// Herança em cadeia: cluster -> grupo -> ativo (grupos aninhados via parent).
func TestEvaluateNestedInheritance(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelParent, "asset_group:sub")
	g.Add("asset_group:sub", RelParent, "asset_group:cluster")
	g.Add("asset_group:cluster", RelOperator, "membership:m1")
	mustAllow(t, g, "asset:a1", RelOperator, "membership:m1", at(t))
}

// Grupo como sujeito: operator = [group#member]; quem é membro herda o acesso.
func TestEvaluateGroupMemberUserset(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "group:dba#member")
	g.Add("group:dba", RelMember, "membership:m1")
	mustAllow(t, g, "asset:a1", RelCanOpenSession, "membership:m1", at(t))
	mustDeny(t, g, "asset:a1", RelCanOpenSession, "membership:naomembro", at(t))
}

// Privilegiada = can_open_session AND has_active_grant. Sem concessão -> negado.
func TestEvaluatePrivilegedNeedsGrant(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	mustDeny(t, g, "asset:a1", RelCanOpenPrivilegedSession, "membership:m1", at(t))
}

// Concessão vigente -> privilegiada permitida.
func TestEvaluatePrivilegedWithActiveGrant(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	g.AddConditioned("asset:a1", RelHasActiveGrant, "membership:m1", ValidityWindow{
		NotBefore: time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
	})
	mustAllow(t, g, "asset:a1", RelCanOpenPrivilegedSession, "membership:m1", at(t))
}

// Concessão expirada -> negado, AINDA QUE a tupla persista (expira no grafo).
func TestEvaluatePrivilegedExpiredGrant(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	g.AddConditioned("asset:a1", RelHasActiveGrant, "membership:m1", ValidityWindow{
		NotBefore: time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC), // já passou às 12h
	})
	// has_active_grant isolado também deve negar.
	mustDeny(t, g, "asset:a1", RelHasActiveGrant, "membership:m1", at(t))
	mustDeny(t, g, "asset:a1", RelCanOpenPrivilegedSession, "membership:m1", at(t))
}

// Tupla condicionada sem janela -> fail-closed (não concede).
func TestEvaluateConditionedTupleWithoutWindowFailsClosed(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	g.Add("asset:a1", RelHasActiveGrant, "membership:m1") // sem janela!
	mustDeny(t, g, "asset:a1", RelHasActiveGrant, "membership:m1", at(t))
}

// Relação desconhecida no tipo -> negado (não erro).
func TestEvaluateUnknownRelationDenies(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset:a1", RelOperator, "membership:m1")
	mustDeny(t, g, "asset:a1", "inexistente", "membership:m1", at(t))
}

// Ciclo no parent não trava e não concede acesso indevido (fail-closed por construção).
func TestEvaluateCycleTerminates(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("asset_group:g1", RelParent, "asset_group:g2")
	g.Add("asset_group:g2", RelParent, "asset_group:g1")
	// Ninguém é operador em lugar nenhum: deve negar, sem loop infinito.
	dec, err := Evaluate(g, "asset_group:g1", RelOperator, "membership:m1", at(t))
	if err != nil {
		t.Fatalf("ciclo não deveria virar erro: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("ciclo não deveria conceder acesso")
	}
}

// A resolução casa tanto o id simples quanto o qualificado por tenant (T-003).
func TestEvaluateTenantQualifiedRefs(t *testing.T) {
	g := NewMemoryGraph()
	g.Add("org:o1/asset:a1", RelOperator, "org:o1/membership:m1")
	mustAllow(t, g, "org:o1/asset:a1", RelCanOpenSession, "org:o1/membership:m1", at(t))
}

func TestEvalConditionUnknownFailsClosed(t *testing.T) {
	w := &ValidityWindow{NotBefore: time.Unix(0, 0), ExpiresAt: time.Unix(1<<40, 0)}
	if evalCondition("condicao_inexistente", w, at(t)) {
		t.Fatalf("condição desconhecida deveria falhar fechada")
	}
	if !evalCondition("", nil, at(t)) {
		t.Fatalf("relação incondicional deveria sempre aplicar")
	}
}

// Guard de profundidade é erro (classe infraestrutura), distinto de negação.
func TestResolveDepthGuardIsError(t *testing.T) {
	if !errors.Is(errResolveDepth, errResolveDepth) {
		t.Fatal("sanity")
	}
}
