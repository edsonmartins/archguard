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

import "testing"

func tup(u, r, o string) RelationTuple { return RelationTuple{User: u, Relation: r, Object: o} }

// Divergência restritiva: tupla no store não prevista pelo banco é "extra" (a
// remover). Divergência ampliativa: tupla esperada ausente é "missing" (só alerta).
func TestReconcileDiffAsymmetry(t *testing.T) {
	expected := []RelationTuple{
		tup("membership:m1", "operator", "asset:a1"),
		tup("membership:m2", "operator", "asset:a2"), // esperada, mas ausente do store
	}
	actual := []RelationTuple{
		tup("membership:m1", "operator", "asset:a1"),
		tup("membership:hacker", "operator", "asset:a1"), // no store, NÃO prevista
	}
	extra, missing := ReconcileDiff(expected, actual)

	if len(extra) != 1 || extra[0].User != "membership:hacker" {
		t.Fatalf("a tupla não prevista deveria ser extra (a remover): %+v", extra)
	}
	if len(missing) != 1 || missing[0].User != "membership:m2" {
		t.Fatalf("a tupla esperada ausente deveria ser missing (alertar): %+v", missing)
	}
}

func TestReconcileDiffNoDivergence(t *testing.T) {
	set := []RelationTuple{tup("membership:m1", "operator", "asset:a1")}
	extra, missing := ReconcileDiff(set, set)
	if len(extra) != 0 || len(missing) != 0 {
		t.Fatalf("estados iguais não deveriam divergir: extra=%v missing=%v", extra, missing)
	}
}

func TestReconcileReportDiverged(t *testing.T) {
	if (ReconcileReport{}).Diverged() {
		t.Fatalf("relatório vazio não divergiu")
	}
	if !(ReconcileReport{Removed: []RelationTuple{tup("a", "b", "c")}}).Diverged() {
		t.Fatalf("com remoção deveria ter divergido")
	}
	if !(ReconcileReport{MissingAlerted: []RelationTuple{tup("a", "b", "c")}}).Diverged() {
		t.Fatalf("com alerta deveria ter divergido")
	}
}
