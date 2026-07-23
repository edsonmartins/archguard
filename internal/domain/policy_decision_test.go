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
	"context"
	"errors"
	"testing"
	"time"
)

func TestRelationTupleValid(t *testing.T) {
	full := RelationTuple{User: "membership:m1", Relation: "operator", Object: "org:o1/asset:a1"}
	if !full.Valid() {
		t.Fatalf("tupla completa deveria ser válida")
	}
	for _, bad := range []RelationTuple{
		{Relation: "operator", Object: "org:o1/asset:a1"},
		{User: "membership:m1", Object: "org:o1/asset:a1"},
		{User: "membership:m1", Relation: "operator"},
		{},
	} {
		if bad.Valid() {
			t.Fatalf("tupla incompleta não deveria ser válida: %+v", bad)
		}
	}
}

func TestCheckRequestValid(t *testing.T) {
	ok := CheckRequest{Tuple: RelationTuple{User: "membership:m1", Relation: "can_open_privileged_session", Object: "org:o1/asset:a1"}}
	if !ok.Valid() {
		t.Fatalf("requisição com tupla completa deveria ser válida")
	}
	if (CheckRequest{Tuple: RelationTuple{User: "membership:m1"}}).Valid() {
		t.Fatalf("requisição com tupla incompleta não deveria ser válida")
	}
}

func TestListObjectsRequestValid(t *testing.T) {
	if !(ListObjectsRequest{User: "membership:m1", Relation: "operator", Type: "org:o1/asset"}).Valid() {
		t.Fatalf("consulta reversa completa deveria ser válida")
	}
	if (ListObjectsRequest{User: "membership:m1", Relation: "operator"}).Valid() {
		t.Fatalf("consulta reversa sem tipo não deveria ser válida")
	}
}

func TestTupleOpValid(t *testing.T) {
	if !TupleWrite.Valid() || !TupleDelete.Valid() {
		t.Fatalf("write/delete deveriam ser operações válidas")
	}
	if TupleOp("upsert").Valid() {
		t.Fatalf("operação desconhecida não deveria ser válida")
	}
}

// A distinção COMPUTADA allowed/denied é o núcleo da porta: uma decisão negada é
// um veredito (err==nil), separada de uma falha de infraestrutura (error).
func TestDecisionAllowDeny(t *testing.T) {
	allow := Allow("operator from parent asset_group:g1")
	if !allow.Allowed || allow.Denied() {
		t.Fatalf("Allow deveria permitir")
	}
	if allow.Justification == "" {
		t.Fatalf("a justificativa deveria acompanhar a decisão (anexada à auditoria)")
	}
	deny := DenyDecision("sem relação")
	if deny.Allowed || !deny.Denied() {
		t.Fatalf("DenyDecision deveria negar")
	}
}

// pdpAlwaysDown é um PDP fictício que nunca alcança veredito. Comprova, em nível
// de tipo, o contrato fail-closed: indisponibilidade retorna ErrPDPUnavailable —
// jamais um Allow — e o chamador trata como negação (INV-6 / RFC-0004 §6).
type pdpAlwaysDown struct{}

func (pdpAlwaysDown) Check(context.Context, CheckRequest) (Decision, error) {
	return Decision{}, ErrPDPUnavailable
}
func (pdpAlwaysDown) ListObjects(context.Context, ListObjectsRequest) ([]string, error) {
	return nil, ErrPDPUnavailable
}
func (pdpAlwaysDown) Write(context.Context, []TupleUpdate) error { return ErrPDPUnavailable }
func (pdpAlwaysDown) Read(context.Context, TupleFilter) ([]RelationTuple, error) {
	return nil, ErrPDPUnavailable
}

var _ PolicyDecisionPoint = pdpAlwaysDown{}

func TestPDPUnavailableIsDenial(t *testing.T) {
	var pdp PolicyDecisionPoint = pdpAlwaysDown{}
	req := CheckRequest{
		Tuple:   RelationTuple{User: "membership:m1", Relation: "can_open_privileged_session", Object: "org:o1/asset:a1"},
		Context: CheckContext{ACR: "L2", EvaluatedAt: time.Unix(0, 0).UTC()},
	}
	dec, err := pdp.Check(context.Background(), req)
	if !errors.Is(err, ErrPDPUnavailable) {
		t.Fatalf("PDP fora do ar deveria retornar ErrPDPUnavailable, veio %v", err)
	}
	// O veredito de zero-valor NUNCA permite: um erro jamais deve ser lido como allow.
	if dec.Allowed {
		t.Fatalf("decisão sob falha jamais deveria permitir (fail-closed)")
	}
}
