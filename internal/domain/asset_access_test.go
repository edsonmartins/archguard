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

// TestNewAssetAccessAssignment: valida a relação (operator/auditor — não pode atribuir
// can_open_* derivada) e o tipo de objeto (asset/asset_group), e gera os refs de grafo.
func TestNewAssetAccessAssignment(t *testing.T) {
	org, subj, obj := uuid.New(), uuid.New(), uuid.New()

	a, err := NewAssetAccessAssignment(org, subj, RelOperator, TypeAsset, obj)
	if err != nil {
		t.Fatalf("operator sobre asset deveria valer: %v", err)
	}
	if a.SubjectRef() != Qualify(org, TypeMembership, subj.String()) {
		t.Errorf("subjectRef = %q", a.SubjectRef())
	}
	if a.ObjectRef() != Qualify(org, TypeAsset, obj.String()) {
		t.Errorf("objectRef = %q", a.ObjectRef())
	}
	if upd, e := a.Tuple(true); e != nil || upd.Op != TupleWrite || upd.Tuple.Relation != RelOperator {
		t.Errorf("tuple = %+v err=%v", upd, e)
	}

	// Relação derivada não é atribuível.
	if _, err := NewAssetAccessAssignment(org, subj, RelCanOpenSession, TypeAsset, obj); !errors.Is(err, ErrInvalidAssetAccess) {
		t.Error("can_open_session não deveria ser atribuível")
	}
	// Objeto inválido.
	if _, err := NewAssetAccessAssignment(org, subj, RelOperator, TypeMembership, obj); !errors.Is(err, ErrInvalidAssetAccess) {
		t.Error("objeto membership não deveria valer (asset/asset_group)")
	}
	// Campos obrigatórios.
	if _, err := NewAssetAccessAssignment(org, uuid.Nil, RelOperator, TypeAsset, obj); !errors.Is(err, ErrInvalidAssetAccess) {
		t.Error("sujeito nulo deveria falhar")
	}
}
