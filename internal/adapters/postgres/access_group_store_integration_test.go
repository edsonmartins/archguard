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

// TestAccessGroupStoreCreateAndList: nomeia um grupo e o lista (metadado, sem projeção).
func TestAccessGroupStoreCreateAndList(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-ag-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM access_group WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	name := "dba-" + uniqueSuffix()
	g, _ := domain.NewAccessGroup(orgID, name, "DBAs")
	if err := NewAccessGroupCatalog(pool).CreateInTenant(ctx, orgID, g); err != nil {
		t.Fatalf("CreateInTenant: %v", err)
	}
	list, err := NewAccessGroupCatalog(pool).ListInTenant(ctx, orgID)
	if err != nil {
		t.Fatalf("ListInTenant: %v", err)
	}
	if len(list) != 1 || list[0].Name != name || list[0].DisplayName != "DBAs" {
		t.Errorf("lista = %+v", list)
	}
}
