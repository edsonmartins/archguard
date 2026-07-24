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
)

// Declarative model tests (T-015 / RFC-0004 §8): each scenario is a small set of
// tuples and one check with an expected verdict — the living specification of the
// authorization model, executed against the same resolver the PDP uses. Adding a
// requirement to the model means adding a row here.

type modelTuple struct {
	object, relation, subject string
	window                    *ValidityWindow
}

type modelScenario struct {
	name                      string
	tuples                    []modelTuple
	object, relation, subject string
	want                      bool
}

func win(nbHour, expHour int) *ValidityWindow {
	return &ValidityWindow{
		NotBefore: time.Date(2026, 7, 23, nbHour, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, expHour, 0, 0, 0, time.UTC),
	}
}

func TestAuthzModelScenarios(t *testing.T) {
	const (
		asset  = "asset:a1"
		group  = "asset_group:g1"
		parent = "asset_group:cluster"
		team   = "group:dba"
		op     = "membership:op"
		other  = "membership:other"
	)
	noon := CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}

	scenarios := []modelScenario{
		{
			name:   "operador direto abre sessão comum",
			tuples: []modelTuple{{asset, RelOperator, op, nil}},
			object: asset, relation: RelCanOpenSession, subject: op, want: true,
		},
		{
			name:   "dono abre sessão comum",
			tuples: []modelTuple{{asset, RelOwner, op, nil}},
			object: asset, relation: RelCanOpenSession, subject: op, want: true,
		},
		{
			name:   "auditor NÃO abre sessão",
			tuples: []modelTuple{{asset, RelAuditor, op, nil}},
			object: asset, relation: RelCanOpenSession, subject: op, want: false,
		},
		{
			name:   "sem relação nega",
			tuples: []modelTuple{{asset, RelOperator, other, nil}},
			object: asset, relation: RelCanOpenSession, subject: op, want: false,
		},
		{
			name: "operador herdado do grupo pai",
			tuples: []modelTuple{
				{asset, RelParent, group, nil},
				{group, RelOperator, op, nil},
			},
			object: asset, relation: RelOperator, subject: op, want: true,
		},
		{
			name: "herança em cadeia (cluster→grupo→ativo)",
			tuples: []modelTuple{
				{asset, RelParent, group, nil},
				{group, RelParent, parent, nil},
				{parent, RelOperator, op, nil},
			},
			object: asset, relation: RelCanOpenSession, subject: op, want: true,
		},
		{
			name: "operador via grupo (group#member)",
			tuples: []modelTuple{
				{asset, RelOperator, team + "#" + RelMember, nil},
				{team, RelMember, op, nil},
			},
			object: asset, relation: RelCanOpenSession, subject: op, want: true,
		},
		{
			name: "não-membro do grupo não herda",
			tuples: []modelTuple{
				{asset, RelOperator, team + "#" + RelMember, nil},
				{team, RelMember, other, nil},
			},
			object: asset, relation: RelCanOpenSession, subject: op, want: false,
		},
		{
			name:   "privilegiada exige concessão",
			tuples: []modelTuple{{asset, RelOperator, op, nil}},
			object: asset, relation: RelCanOpenPrivilegedSession, subject: op, want: false,
		},
		{
			name: "privilegiada com concessão vigente",
			tuples: []modelTuple{
				{asset, RelOperator, op, nil},
				{asset, RelHasActiveGrant, op, win(11, 13)},
			},
			object: asset, relation: RelCanOpenPrivilegedSession, subject: op, want: true,
		},
		{
			name: "privilegiada com concessão expirada nega",
			tuples: []modelTuple{
				{asset, RelOperator, op, nil},
				{asset, RelHasActiveGrant, op, win(8, 9)},
			},
			object: asset, relation: RelCanOpenPrivilegedSession, subject: op, want: false,
		},
		{
			name: "concessão sem acesso estrutural não abre privilegiada",
			tuples: []modelTuple{
				{asset, RelHasActiveGrant, op, win(11, 13)},
			},
			object: asset, relation: RelCanOpenPrivilegedSession, subject: op, want: false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			g := NewMemoryGraph()
			for _, tp := range s.tuples {
				if tp.window != nil {
					g.AddConditioned(tp.object, tp.relation, tp.subject, *tp.window)
				} else {
					g.Add(tp.object, tp.relation, tp.subject)
				}
			}
			dec, err := Evaluate(g, s.object, s.relation, s.subject, noon)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if dec.Allowed != s.want {
				t.Fatalf("veredito %v, esperado %v (justificativa: %q)", dec.Allowed, s.want, dec.Justification)
			}
		})
	}
}
