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

// TestReconcileRemovesStaleRevokedMembershipTuple (M4 Fase F, T-031 / T-030c): o reconciler
// remove do grafo a tupla de acesso de um membership REVOGADO (que um caminho de bypass
// deixou obsoleta) e PRESERVA a de um membership ativo. É a rede de segurança do T-030c.
func TestReconcileRemovesStaleRevokedMembershipTuple(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		"INSERT INTO organization (owner, name) VALUES ('it', $1) RETURNING id", "org-rec-"+uniqueSuffix()).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	idn, _ := domain.NewIdentity(domain.IdentityHuman)
	if err := NewIdentityStore(pool).Create(ctx, idn); err != nil {
		t.Fatalf("cria identidade: %v", err)
	}
	// duas identidades (o par identity+org é único), uma ativa e uma revogada.
	idn2, _ := domain.NewIdentity(domain.IdentityHuman)
	if err := NewIdentityStore(pool).Create(ctx, idn2); err != nil {
		t.Fatalf("cria identidade 2: %v", err)
	}
	mActive, _ := domain.NewMembership(idn.ID, orgID)
	mRevoked, _ := domain.NewMembership(idn2.ID, orgID)

	assetA, assetR := uuid.New(), uuid.New()
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, "DELETE FROM authz_tuple WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM asset WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM membership WHERE organization_id = $1", orgID)
		_, _ = pool.Exec(bg, "DELETE FROM organization WHERE id = $1", orgID)
	})

	// mActive (active) possui assetA; mRevoked (revoked) possui assetR.
	if _, err := pool.Exec(ctx, "INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,'active')",
		mActive.ID.String(), mActive.IdentityID.String(), orgID.String()); err != nil {
		t.Fatalf("insert membership ativo: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1,$2,$3,'revoked')",
		mRevoked.ID.String(), mRevoked.IdentityID.String(), orgID.String()); err != nil {
		t.Fatalf("insert membership revogado: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO asset (id, organization_id, kind, name, owner_membership_id) VALUES ($1,$2,'host','a',$3)",
		assetA.String(), orgID.String(), mActive.ID.String()); err != nil {
		t.Fatalf("insert assetA: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO asset (id, organization_id, kind, name, owner_membership_id) VALUES ($1,$2,'host','r',$3)",
		assetR.String(), orgID.String(), mRevoked.ID.String()); err != nil {
		t.Fatalf("insert assetR: %v", err)
	}

	ownerActive := domain.RelationTuple{User: domain.Qualify(orgID, domain.TypeMembership, mActive.ID.String()), Relation: domain.RelOwner, Object: domain.Qualify(orgID, domain.TypeAsset, assetA.String())}
	ownerStale := domain.RelationTuple{User: domain.Qualify(orgID, domain.TypeMembership, mRevoked.ID.String()), Relation: domain.RelOwner, Object: domain.Qualify(orgID, domain.TypeAsset, assetR.String())}
	bogus := domain.RelationTuple{User: domain.Qualify(orgID, domain.TypeMembership, uuid.New().String()), Relation: domain.RelOperator, Object: domain.Qualify(orgID, domain.TypeAsset, uuid.New().String())}

	// Projeção já contém: a válida (mActive) + duas obsoletas (mRevoked + bogus).
	for _, tup := range []domain.RelationTuple{ownerActive, ownerStale, bogus} {
		if _, err := pool.Exec(ctx,
			"INSERT INTO authz_tuple (organization_id, tuple_user, tuple_relation, tuple_object) VALUES ($1,$2,$3,$4)",
			orgID.String(), tup.User, tup.Relation, tup.Object); err != nil {
			t.Fatalf("seed authz_tuple: %v", err)
		}
	}

	report, err := NewReconcileService(pool).ReconcileTenant(ctx, orgID)
	if err != nil {
		t.Fatalf("ReconcileTenant: %v", err)
	}
	if len(report.Removed) != 2 {
		t.Errorf("esperava 2 tuplas removidas (mRevoked owner + bogus), veio %d: %+v", len(report.Removed), report.Removed)
	}

	has := func(tup domain.RelationTuple) bool {
		var n int
		_ = pool.QueryRow(ctx,
			"SELECT count(*) FROM authz_tuple WHERE organization_id=$1 AND tuple_user=$2 AND tuple_relation=$3 AND tuple_object=$4",
			orgID.String(), tup.User, tup.Relation, tup.Object).Scan(&n)
		return n > 0
	}
	if !has(ownerActive) {
		t.Error("a tupla do membership ATIVO não deveria ser removida")
	}
	if has(ownerStale) {
		t.Error("a tupla do membership REVOGADO deveria ter sido removida (T-030c)")
	}
	if has(bogus) {
		t.Error("a tupla espúria deveria ter sido removida")
	}
}
