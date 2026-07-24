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
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

func newTenantConnector(t *testing.T, org domain.TenantScope) domain.DirectoryConnector {
	t.Helper()
	c, err := domain.NewDirectoryConnector(org.OrganizationID(), domain.DirectoryAD, "AD Corp",
		"(&(objectClass=user)(memberOf=CN=ArchGuard))", "vault://kv/data/org/ad-bind",
		[]domain.AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}},
		[]domain.GroupMapping{{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: true}})
	if err != nil {
		t.Fatalf("NewDirectoryConnector: %v", err)
	}
	return c
}

// Cria e recarrega um conector, preservando o mapeamento versionado e a ref de
// credencial (nunca o segredo).
func TestDirectoryConnectorCreateAndGet(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeTenant(t, pool, "dc")
	repo := NewTenantRepository(pool, fx.scope)
	conn := newTenantConnector(t, fx.scope)

	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewDirectoryConnectorStore(ttx).Create(ctx, conn)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got domain.DirectoryConnector
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		got, e = NewDirectoryConnectorStore(ttx).Get(ctx, conn.ID)
		return e
	}); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ScopeFilter != conn.ScopeFilter || got.CredentialRef != conn.CredentialRef {
		t.Fatalf("escopo/credencial não preservados: %+v", got)
	}
	if got.Enabled {
		t.Fatalf("conector deveria estar desabilitado por padrão")
	}
	if got.Mapping.Version != 1 || len(got.Mapping.Attributes) != 1 || len(got.Mapping.Groups) != 1 {
		t.Fatalf("mapeamento versionado não preservado: %+v", got.Mapping)
	}
	if !got.Mapping.Groups[0].Approved {
		t.Fatalf("flag Approved do grupo deveria persistir")
	}
}

// RLS/Barreira 1: o conector de um tenant não é visível em outro.
func TestDirectoryConnectorTenantIsolation(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "dc-a")
	b := makeTenant(t, pool, "dc-b")
	conn := newTenantConnector(t, a.scope)

	if err := NewTenantRepository(pool, a.scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewDirectoryConnectorStore(ttx).Create(ctx, conn)
	}); err != nil {
		t.Fatalf("Create em A: %v", err)
	}

	// B não enxerga o conector de A.
	err := NewTenantRepository(pool, b.scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		_, e := NewDirectoryConnectorStore(ttx).Get(ctx, conn.ID)
		return e
	})
	if !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("tenant B não deveria ver o conector de A, veio %v", err)
	}
}

// Barreira 1 na escrita: a store recusa persistir conector de outro tenant.
func TestDirectoryConnectorRejectsCrossTenantWrite(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	a := makeTenant(t, pool, "dc-wa")
	b := makeTenant(t, pool, "dc-wb")
	// Conector pertencente a B, gravado pela store de A.
	connB := newTenantConnector(t, b.scope)

	err := NewTenantRepository(pool, a.scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewDirectoryConnectorStore(ttx).Create(ctx, connB)
	})
	if !errors.Is(err, ErrCrossTenantWrite) {
		t.Fatalf("escrita cross-tenant deveria ser recusada, veio %v", err)
	}
}
