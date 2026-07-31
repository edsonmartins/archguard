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

// TestAssetStoreCreateAndProject: cria um grupo e um ativo (parent = grupo) e verifica
// que (1) List/ListGroups os retornam e (2) o AuthzOutbox recebeu as tuplas estruturais
// derivadas — na MESMA transação (RFC-0004 §4, T-026).
func TestAssetStoreCreateAndProject(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-asset-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset_group WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	repo := NewTenantRepository(pool, scope)

	group, err := domain.NewAssetGroup(orgID, "prod-db", nil)
	if err != nil {
		t.Fatalf("NewAssetGroup: %v", err)
	}
	asset, err := domain.NewAsset(orgID, "host", "db-prod-03", "", &group.ID, nil)
	if err != nil {
		t.Fatalf("NewAsset: %v", err)
	}

	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		s := NewAssetStore(ttx)
		if e := s.CreateGroup(ctx, group); e != nil {
			return e
		}
		return s.Create(ctx, asset)
	}); err != nil {
		t.Fatalf("criação: %v", err)
	}

	// List retorna o ativo com o parent correto.
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		assets, e := NewAssetStore(ttx).List(ctx)
		if e != nil {
			return e
		}
		if len(assets) != 1 || assets[0].Name != "db-prod-03" || assets[0].ParentGroupID == nil || *assets[0].ParentGroupID != group.ID {
			t.Errorf("assets = %+v", assets)
		}
		groups, e := NewAssetStore(ttx).ListGroups(ctx)
		if e != nil {
			return e
		}
		if len(groups) != 1 || groups[0].Name != "prod-db" {
			t.Errorf("groups = %+v", groups)
		}
		return nil
	}); err != nil {
		t.Fatalf("listagem: %v", err)
	}

	// O outbox recebeu a tupla `parent` do ativo (asset → group). A projeção é atômica
	// com a criação, então já está lá sem o publisher rodar.
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id = $1 AND tuple_relation = 'parent' AND tuple_object = $2",
		orgID, asset.Ref()).Scan(&n); err != nil {
		t.Fatalf("contando outbox: %v", err)
	}
	if n == 0 {
		t.Error("esperava tupla `parent` do ativo no outbox — projeção não foi enfileirada na mesma tx")
	}
}

// TestAssetProjectionPublishesToTuple: o pipeline completo do M4 (Fase A+C) — criar um
// ativo enfileira a projeção no outbox, e o TuplePublisher a drena para authz_tuple, de
// onde o PDP decide. Prova que outbox → publisher → tuple funciona fim a fim.
func TestAssetProjectionPublishesToTuple(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-proj-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset_group WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	group, _ := domain.NewAssetGroup(orgID, "grp-"+uniqueSuffix(), nil)
	asset, _ := domain.NewAsset(orgID, "host", "h-"+uniqueSuffix(), "", &group.ID, nil)
	if err := NewTenantRepository(pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		s := NewAssetStore(ttx)
		if e := s.CreateGroup(ctx, group); e != nil {
			return e
		}
		return s.Create(ctx, asset)
	}); err != nil {
		t.Fatalf("criação: %v", err)
	}

	// Drena o outbox para a projeção.
	published, err := NewTuplePublisher().Publish(ctx, pool, 100)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if published == 0 {
		t.Fatal("Publish não drenou nada — o enqueue não chegou ao outbox")
	}

	// A tupla `parent` do ativo agora vive em authz_tuple (o PDP a enxerga).
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM authz_tuple WHERE organization_id = $1 AND tuple_relation = 'parent' AND tuple_object = $2",
		orgID, asset.Ref()).Scan(&n); err != nil {
		t.Fatalf("consultando authz_tuple: %v", err)
	}
	if n == 0 {
		t.Error("esperava a tupla `parent` do ativo em authz_tuple após o publish")
	}
}
