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
	"testing"
	"time"

	"github.com/google/uuid"
)

// A campanha de revisão sobre um ativo lista os memberships com acesso efetivo e
// classifica a origem: direto, herdado (from parent) e por concessão.
func TestReviewAssetAccessOrigins(t *testing.T) {
	org := uuid.New()
	asset := Qualify(org, TypeAsset, "a1")
	parentGroup := Qualify(org, TypeAssetGroup, "cluster")

	memDirect := Qualify(org, TypeMembership, "direct")
	memInherited := Qualify(org, TypeMembership, "inherited")
	memGrant := Qualify(org, TypeMembership, "grant")

	g := NewMemoryGraph()
	// Direto: operator no próprio ativo.
	g.Add(asset, RelOperator, memDirect)
	// Herdado: operator no grupo pai do ativo.
	g.Add(asset, RelParent, parentGroup)
	g.Add(parentGroup, RelOperator, memInherited)
	// Por concessão: operator direto + concessão vigente.
	g.Add(asset, RelOperator, memGrant)
	g.AddConditioned(asset, RelHasActiveGrant, memGrant, ValidityWindow{
		NotBefore: time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
	})
	// Alguém sem acesso nenhum não deve aparecer.
	memNone := Qualify(org, TypeMembership, "none")

	ctx := CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}
	entries, err := ReviewAssetAccess(g, asset,
		[]string{memDirect, memInherited, memGrant, memNone}, ctx)
	if err != nil {
		t.Fatalf("ReviewAssetAccess: %v", err)
	}

	bySubject := map[string]AccessReviewEntry{}
	for _, e := range entries {
		bySubject[e.Subject] = e
	}
	if len(bySubject) != 3 {
		t.Fatalf("esperava 3 memberships com acesso efetivo, veio %d", len(bySubject))
	}
	if _, ok := bySubject[memNone]; ok {
		t.Fatalf("quem não tem acesso não deveria aparecer na revisão")
	}
	if e := bySubject[memDirect]; !e.HasOrigin(OriginDirect) || e.HasOrigin(OriginInherited) || e.HasOrigin(OriginGrant) {
		t.Fatalf("acesso direto mal classificado: %+v", e.Origins)
	}
	if e := bySubject[memInherited]; !e.HasOrigin(OriginInherited) || e.HasOrigin(OriginGrant) {
		t.Fatalf("acesso herdado mal classificado: %+v", e.Origins)
	}
	if e := bySubject[memGrant]; !e.HasOrigin(OriginDirect) || !e.HasOrigin(OriginGrant) {
		t.Fatalf("acesso por concessão deveria ter origem direto+concessão: %+v", e.Origins)
	}
}

// Fora da janela, a concessão não conta como origem (expira no grafo), mas o
// acesso estrutural (operator) permanece.
func TestReviewAssetAccessGrantExpires(t *testing.T) {
	org := uuid.New()
	asset := Qualify(org, TypeAsset, "a1")
	mem := Qualify(org, TypeMembership, "m")

	g := NewMemoryGraph()
	g.Add(asset, RelOperator, mem)
	g.AddConditioned(asset, RelHasActiveGrant, mem, ValidityWindow{
		NotBefore: time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC),
	})
	ctx := CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}
	entries, err := ReviewAssetAccess(g, asset, []string{mem}, ctx)
	if err != nil {
		t.Fatalf("ReviewAssetAccess: %v", err)
	}
	if len(entries) != 1 || entries[0].HasOrigin(OriginGrant) {
		t.Fatalf("concessão expirada não deveria contar como origem: %+v", entries)
	}
	if !entries[0].HasOrigin(OriginDirect) {
		t.Fatalf("o acesso estrutural deveria permanecer: %+v", entries)
	}
}
