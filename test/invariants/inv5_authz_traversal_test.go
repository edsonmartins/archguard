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

package invariants

// INV-5 / I-6.3 no GRAFO de autorização (pacote 007, T-016 / spec "Isolamento de
// tenant no grafo"): nenhuma relação concede acesso a objeto de outro tenant. Duas
// barreiras independentes, ambas puras (rodam sem PostgreSQL, parte do gate):
//
//   - Barreira de ESCRITA: ValidateTuple recusa qualquer tupla cujo sujeito e
//     objeto não sejam do mesmo tenant — a tupla cruzada nunca entra no store.
//   - Barreira de DECISÃO: mesmo que uma tupla cruzada seja injetada no store por
//     outro caminho, o portão GuardSameTenant nega a consulta ANTES de resolver o
//     grafo. Provamos isso injetando a tupla proibida diretamente no grafo.
//
// Falha aqui quebra o build (make invariants).

import (
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// pdpDecision replica o caminho de decisão do PDP (portão + resolvedor) sem banco:
// GuardSameTenant primeiro, depois Evaluate. É o que PostgresPDP.Check faz.
func pdpDecision(g domain.GraphReader, user, relation, object string) (domain.Decision, error) {
	if err := domain.GuardSameTenant(user, object); err != nil {
		return domain.DenyDecision("cross-tenant"), nil
	}
	return domain.Evaluate(g, object, relation, user, domain.CheckContext{})
}

// Barreira de escrita: nenhuma tupla cruzando organizações é aceita.
func TestINV5AuthzWriteBarrier(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	cross := domain.RelationTuple{
		User:     domain.Qualify(orgA, domain.TypeMembership, uuid.New().String()),
		Relation: domain.RelOperator,
		Object:   domain.Qualify(orgB, domain.TypeAsset, uuid.New().String()),
	}
	if err := domain.ValidateTuple(cross); !errors.Is(err, domain.ErrCrossTenantRelation) {
		t.Fatalf("tupla cruzando tenants deveria ser recusada na escrita, veio %v", err)
	}
}

// Barreira de decisão: uma tupla cruzada INJETADA no store não concede acesso —
// o portão nega antes de o resolvedor sequer olhar o grafo envenenado.
func TestINV5AuthzDecisionBarrierEvenWhenPoisoned(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	attacker := domain.Qualify(orgA, domain.TypeMembership, "atacante")
	victimAsset := domain.Qualify(orgB, domain.TypeAsset, "cofre")

	// Envenena o grafo: concede operator direto ao membership do tenant A sobre o
	// ativo do tenant B (o que a escrita jamais permitiria).
	g := domain.NewMemoryGraph()
	g.Add(victimAsset, domain.RelOperator, attacker)

	// Sem o portão, o resolvedor puro CONCEDERIA (a tupla existe) — prova de que a
	// barreira de decisão é indispensável.
	if dec, _ := domain.Evaluate(g, victimAsset, domain.RelCanOpenSession, attacker, domain.CheckContext{}); !dec.Allowed {
		t.Fatalf("sanidade: o grafo envenenado deveria conceder sem o portão")
	}

	// Com o caminho de decisão do PDP (portão + resolvedor), é NEGADO.
	dec, err := pdpDecision(g, attacker, domain.RelCanOpenSession, victimAsset)
	if err != nil {
		t.Fatalf("cross-tenant deveria ser negação computada, não erro: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("nenhuma relação pode conceder acesso a objeto de outro tenant")
	}
}

// Consulta legítima intra-tenant continua funcionando (o portão não é bloqueio cego).
func TestINV5AuthzIntraTenantStillWorks(t *testing.T) {
	org := uuid.New()
	mem := domain.Qualify(org, domain.TypeMembership, "m")
	asset := domain.Qualify(org, domain.TypeAsset, "a")
	g := domain.NewMemoryGraph()
	g.Add(asset, domain.RelOperator, mem)
	dec, err := pdpDecision(g, mem, domain.RelCanOpenSession, asset)
	if err != nil || !dec.Allowed {
		t.Fatalf("acesso intra-tenant legítimo deveria ser permitido: allowed=%v err=%v", dec.Allowed, err)
	}
}
