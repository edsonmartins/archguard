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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// projectAndPublish enfileira os updates e os drena para o store (ponta a ponta:
// projeção → outbox → publisher → authz_tuple), inclusive tuplas condicionadas.
func projectAndPublish(t *testing.T, pool *pgxpool.Pool, updates []domain.TupleUpdate) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := NewAuthzOutbox(tx).Enqueue(ctx, updates); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := NewTuplePublisher().Publish(ctx, pool, 1000); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

// Decisão privilegiada ponta a ponta, SEM cache: operador via grupo + concessão
// vigente abrem a privilegiada dentro da janela; a MESMA consulta fora da janela
// nega (a concessão expira no grafo); a sessão comum segue permitida.
func TestPostgresPDPPrivilegedDecision(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	assetID, groupID, mem := uuid.New(), uuid.New(), uuid.New()
	assetRef := domain.Qualify(org, domain.TypeAsset, assetID.String())
	memRef := domain.Qualify(org, domain.TypeMembership, mem.String())
	groupUserset := domain.QualifyUserset(org, domain.TypeGroup, groupID.String(), domain.RelMember)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", org.String())
	})

	roleTuple, err := domain.ProjectRoleAssignment(assetRef, domain.RelOperator, groupUserset, true)
	if err != nil {
		t.Fatalf("ProjectRoleAssignment: %v", err)
	}
	grant := domain.PrivilegedGrant{
		ID: uuid.New(), OrganizationID: org, SubjectMembershipID: mem, Status: domain.GrantActive,
		NotBefore: time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
	}
	grantTuple, err := domain.ProjectGrant(grant, assetRef)
	if err != nil {
		t.Fatalf("ProjectGrant: %v", err)
	}
	projectAndPublish(t, pool, []domain.TupleUpdate{
		roleTuple,
		domain.ProjectGroupMembership(org, groupID, mem, true),
		grantTuple,
	})

	pdp := NewPostgresPDP(pool)

	within := domain.CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)}
	if dec, err := pdp.CanOpenPrivilegedSession(ctx, memRef, assetRef, within); err != nil || !dec.Allowed {
		t.Fatalf("dentro da janela a privilegiada deveria ser permitida: allowed=%v err=%v", dec.Allowed, err)
	}

	after := domain.CheckContext{ACR: "L2", EvaluatedAt: time.Date(2026, 7, 23, 13, 0, 1, 0, time.UTC)}
	if dec, err := pdp.CanOpenPrivilegedSession(ctx, memRef, assetRef, after); err != nil || dec.Allowed {
		t.Fatalf("fora da janela a privilegiada deveria ser negada (expira no grafo): allowed=%v err=%v", dec.Allowed, err)
	}

	// A sessão comum (só operator) segue permitida fora da janela da concessão.
	common := domain.CheckRequest{
		Tuple:   domain.RelationTuple{User: memRef, Relation: domain.RelCanOpenSession, Object: assetRef},
		Context: after,
	}
	if dec, err := pdp.Check(ctx, common); err != nil || !dec.Allowed {
		t.Fatalf("a sessão comum por operator deveria seguir permitida: allowed=%v err=%v", dec.Allowed, err)
	}
}

// Sem qualquer relação -> negado (computado, não erro). spec "Sem relação".
func TestPostgresPDPNoRelationDenies(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	memRef := domain.Qualify(org, domain.TypeMembership, uuid.New().String())
	assetRef := domain.Qualify(org, domain.TypeAsset, uuid.New().String())

	dec, err := NewPostgresPDP(pool).CanOpenPrivilegedSession(ctx, memRef, assetRef, domain.CheckContext{ACR: "L2"})
	if err != nil {
		t.Fatalf("negação por ausência de relação não deveria ser erro: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("sem relação deveria negar")
	}
}

// Consulta cruzada -> negada (spec "Consulta cruzada"), sem tocar o store.
func TestPostgresPDPCrossTenantDenied(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	memA := domain.Qualify(orgA, domain.TypeMembership, uuid.New().String())
	assetB := domain.Qualify(orgB, domain.TypeAsset, uuid.New().String())

	dec, err := NewPostgresPDP(pool).CanOpenPrivilegedSession(ctx, memA, assetB, domain.CheckContext{ACR: "L2"})
	if err != nil {
		t.Fatalf("consulta cruzada deveria ser negação computada, não erro: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("consulta cruzada deveria ser negada")
	}
}

// Campanha de revisão: lista os memberships com acesso efetivo ao ativo e a
// origem de cada um (spec "Campanha de revisão", T-014).
func TestPostgresPDPReviewAsset(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	assetID, groupID := uuid.New(), uuid.New()
	assetRef := domain.Qualify(org, domain.TypeAsset, assetID.String())
	direct := domain.Qualify(org, domain.TypeMembership, uuid.New().String())
	viaGroup := uuid.New()
	viaGroupRef := domain.Qualify(org, domain.TypeMembership, viaGroup.String())
	groupUserset := domain.QualifyUserset(org, domain.TypeGroup, groupID.String(), domain.RelMember)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", org.String())
	})

	directRole, _ := domain.ProjectRoleAssignment(assetRef, domain.RelOperator, direct, true)
	groupRole, _ := domain.ProjectRoleAssignment(assetRef, domain.RelOperator, groupUserset, true)
	projectAndPublish(t, pool, []domain.TupleUpdate{
		directRole, groupRole, domain.ProjectGroupMembership(org, groupID, viaGroup, true),
	})

	entries, err := NewPostgresPDP(pool).ReviewAsset(ctx, assetRef)
	if err != nil {
		t.Fatalf("ReviewAsset: %v", err)
	}
	subjects := map[string]bool{}
	for _, e := range entries {
		subjects[e.Subject] = true
	}
	if !subjects[direct] || !subjects[viaGroupRef] {
		t.Fatalf("revisão deveria incluir o membership direto e o via grupo: %+v", entries)
	}
}

// SEM cache: um delete publicado é refletido na próxima decisão.
func TestPostgresPDPNoCache(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	assetRef := domain.Qualify(org, domain.TypeAsset, uuid.New().String())
	memRef := domain.Qualify(org, domain.TypeMembership, uuid.New().String())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple WHERE organization_id = $1", org.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM authz_tuple_outbox WHERE organization_id = $1", org.String())
	})

	role, _ := domain.ProjectRoleAssignment(assetRef, domain.RelOperator, memRef, true)
	projectAndPublish(t, pool, []domain.TupleUpdate{role})

	pdp := NewPostgresPDP(pool)
	common := domain.CheckRequest{Tuple: domain.RelationTuple{User: memRef, Relation: domain.RelCanOpenSession, Object: assetRef}}
	if dec, _ := pdp.Check(ctx, common); !dec.Allowed {
		t.Fatalf("operator deveria permitir a sessão comum")
	}

	// Revoga o operator (delete) e republica; a decisão seguinte reflete de imediato.
	roleDel, _ := domain.ProjectRoleAssignment(assetRef, domain.RelOperator, memRef, false)
	projectAndPublish(t, pool, []domain.TupleUpdate{roleDel})
	if dec, _ := pdp.Check(ctx, common); dec.Allowed {
		t.Fatalf("após remover o operator a decisão deveria negar (sem cache)")
	}
}
