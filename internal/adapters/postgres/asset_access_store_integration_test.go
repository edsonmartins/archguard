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

package postgres

import (
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// TestAssetAccessStoreCreateAndProject: criar uma atribuição (membership operator sobre
// um asset) grava a linha e enfileira a tupla `operator` no outbox — na mesma tx (T-029).
func TestAssetAccessStoreCreateAndProject(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-aa-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset_access_assignment WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	subj, obj := uuid.New(), uuid.New()
	assignment, err := domain.NewAssetAccessAssignment(orgID, domain.TypeMembership, subj, domain.RelOperator, domain.TypeAsset, obj)
	if err != nil {
		t.Fatalf("NewAssetAccessAssignment: %v", err)
	}
	if err := NewTenantRepository(pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewAssetAccessStore(ttx).Create(ctx, assignment)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A linha persistiu.
	var listed int
	if err := NewTenantRepository(pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		as, e := NewAssetAccessStore(ttx).List(ctx)
		listed = len(as)
		return e
	}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if listed != 1 {
		t.Errorf("esperava 1 atribuição, veio %d", listed)
	}

	// O outbox recebeu a tupla `operator` (subject → operator → asset).
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id = $1 AND tuple_relation = 'operator' AND tuple_object = $2",
		orgID, assignment.ObjectRef()).Scan(&n); err != nil {
		t.Fatalf("contando outbox: %v", err)
	}
	if n == 0 {
		t.Error("esperava a tupla `operator` no outbox — projeção não foi enfileirada na mesma tx")
	}
}
