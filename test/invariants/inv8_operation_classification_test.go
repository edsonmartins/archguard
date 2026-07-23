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

// INV-8 (T-017) — cenário da spec "Operação sem classificação": toda operação da
// API declara nível de garantia (L1/L2/L3). Este teste falha o build se um verbo
// catalogado de auditoria não tiver NEM classificação de operação NEM isenção
// explícita — não há lacuna silenciosa. Adicionar um verbo novo força uma
// decisão: classificá-lo como operação ou isentá-lo com um motivo.

import (
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

// O catálogo canônico de operações constrói sem erro e cada operação declara um
// nível válido.
func TestINV8OperationCatalogBuilds(t *testing.T) {
	cat, err := domain.BuildOperationCatalog()
	if err != nil {
		t.Fatalf("INV-8: catálogo de operações inválido: %v", err)
	}
	ids := cat.IDs()
	if len(ids) == 0 {
		t.Fatalf("INV-8: catálogo de operações vazio")
	}
	for _, id := range ids {
		op, ok := cat.Lookup(id)
		if !ok || !op.Level.Valid() {
			t.Fatalf("INV-8: operação %q sem nível de garantia válido", id)
		}
	}
}

// Completude: TODO verbo de auditoria é classificado como operação OU está na
// isenção explícita — nunca ambos, nunca nenhum. É este teste que rejeita o
// build de uma operação sem classificação.
func TestINV8EveryActionClassifiedOrExempt(t *testing.T) {
	cat, err := domain.BuildOperationCatalog()
	if err != nil {
		t.Fatalf("INV-8: catálogo de operações inválido: %v", err)
	}
	exempt := domain.OperationExemptActions()

	for _, action := range domain.CatalogedActions() {
		_, classified := cat.Lookup(string(action))
		_, isExempt := exempt[action]

		switch {
		case classified && isExempt:
			t.Errorf("INV-8: ação %q está classificada como operação E isenta — escolha uma", action)
		case !classified && !isExempt:
			t.Errorf("INV-8: ação %q não é classificada como operação nem isenta — classifique-a ou isente-a com motivo", action)
		}
	}
}

// Consistência: uma operação cujo id é um verbo de auditoria deve ter o MESMO
// nível do verbo — um endpoint e sua trilha nunca discordam do custo da ação.
func TestINV8OperationLevelMatchesActionLevel(t *testing.T) {
	cat, err := domain.BuildOperationCatalog()
	if err != nil {
		t.Fatalf("INV-8: catálogo de operações inválido: %v", err)
	}
	for _, action := range domain.CatalogedActions() {
		op, ok := cat.Lookup(string(action))
		if !ok {
			continue // isenta — verificada no teste de completude
		}
		if op.Level != action.AssuranceLevel() {
			t.Errorf("INV-8: operação %q é %s mas o verbo de auditoria é %s — devem coincidir",
				action, op.Level, action.AssuranceLevel())
		}
	}
}
