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

// TestGroupAccessChainProjects (M4 D1): vincular um membership a um grupo projeta a tupla
// `member`, e atribuir operator ao GRUPO projeta a relação sobre o userset group#member —
// as duas pontas da cadeia membership→member→group→operator→asset.
func TestGroupAccessChainProjects(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-grp-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	idn, _ := domain.NewIdentity(domain.IdentityHuman)
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	m, _ := domain.NewMembership(idn.ID, orgID)
	if _, err := pool.Exec(ctx, "INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,'active')",
		m.ID.String(), m.IdentityID.String(), orgID.String()); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	groupID, assetID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset_access_assignment WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM group_membership WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	scope, _ := domain.NewTenantScope(orgID)
	repo := NewTenantRepository(pool, scope)

	// 1) vincula o membership ao grupo → tupla `member`.
	gm, _ := domain.NewGroupMembership(orgID, groupID, m.ID)
	// 2) atribui operator ao GRUPO sobre o ativo → tupla no userset group#member.
	asg, _ := domain.NewAssetAccessAssignment(orgID, domain.TypeGroup, groupID, domain.RelOperator, domain.TypeAsset, assetID)
	if err := repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		if e := NewGroupMembershipStore(ttx).Create(ctx, gm); e != nil {
			return e
		}
		return NewAssetAccessStore(ttx).Create(ctx, asg)
	}); err != nil {
		t.Fatalf("criação: %v", err)
	}

	memberRef := domain.Qualify(orgID, domain.TypeMembership, m.ID.String())
	groupRef := domain.Qualify(orgID, domain.TypeGroup, groupID.String())
	userset := groupRef + "#member"

	count := func(user, relation, object string) int {
		var n int
		_ = pool.QueryRow(ctx,
			"SELECT count(*) FROM authz_tuple_outbox WHERE organization_id=$1 AND tuple_user=$2 AND tuple_relation=$3 AND tuple_object=$4 AND op='write'",
			orgID, user, relation, object).Scan(&n)
		return n
	}
	if count(memberRef, "member", groupRef) == 0 {
		t.Error("esperava a tupla `member` (membership→group) no outbox")
	}
	if count(userset, "operator", domain.Qualify(orgID, domain.TypeAsset, assetID.String())) == 0 {
		t.Error("esperava a tupla `operator` do userset group#member sobre o ativo")
	}
}
