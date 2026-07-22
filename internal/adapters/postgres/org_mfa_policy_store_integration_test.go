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

// Uma organização sem política declarada usa o piso-base (AAL1); ao declarar
// AAL3, a leitura e a autoridade passam a exigir WebAuthn.
func TestOrgMFAPolicyStoreSetAndGet(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "mfapol")

	// Sem linha: default AAL1.
	repoA := NewTenantRepository(pool, fx.tenantScopeA)
	var got domain.OrgMFAPolicy
	if err := repoA.WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		got, e = NewOrgMFAPolicyStore(ttx).Get(ctx, fx.orgA)
		return e
	}); err != nil {
		t.Fatalf("Get default: %v", err)
	}
	if got.MinimumAAL != domain.AAL1 {
		t.Fatalf("org sem política deveria ser AAL1, veio %s", got.MinimumAAL)
	}

	// Declara AAL3.
	policy, err := domain.NewOrgMFAPolicy(fx.orgA, domain.AAL3)
	if err != nil {
		t.Fatalf("NewOrgMFAPolicy: %v", err)
	}
	if err := repoA.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewOrgMFAPolicyStore(ttx).Set(ctx, policy)
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// A autoridade (TenantAuthPolicy) reflete o piso declarado.
	req, err := NewOrgPolicyAuthority(pool).RequiredAAL(ctx, fx.orgA)
	if err != nil {
		t.Fatalf("RequiredAAL: %v", err)
	}
	if req != domain.AAL3 {
		t.Fatalf("RequiredAAL(orgA) = %s, quero aal3", req)
	}

	// Outra org, sem política, segue no baseline AAL1.
	reqB, err := NewOrgPolicyAuthority(pool).RequiredAAL(ctx, fx.orgB)
	if err != nil {
		t.Fatalf("RequiredAAL orgB: %v", err)
	}
	if reqB != domain.AAL1 {
		t.Fatalf("orgB sem política deveria ser AAL1, veio %s", reqB)
	}

	// Upsert: baixar para AAL2.
	lowered, _ := domain.NewOrgMFAPolicy(fx.orgA, domain.AAL2)
	if err := repoA.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewOrgMFAPolicyStore(ttx).Set(ctx, lowered)
	}); err != nil {
		t.Fatalf("Set upsert: %v", err)
	}
	req2, _ := NewOrgPolicyAuthority(pool).RequiredAAL(ctx, fx.orgA)
	if req2 != domain.AAL2 {
		t.Fatalf("após upsert RequiredAAL(orgA) = %s, quero aal2", req2)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM organization_mfa_policy WHERE organization_id = $1", fx.orgA.String())
	})
}

// Barreira 1: o store escopado a A recusa ler/escrever a política de B.
func TestOrgMFAPolicyStoreRejectsCrossTenant(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "mfaxt")

	err := NewTenantRepository(pool, fx.tenantScopeA).WithTenantTx(ctx, func(ttx *TenantTx) error {
		if _, e := NewOrgMFAPolicyStore(ttx).Get(ctx, fx.orgB); !errors.Is(e, ErrCrossTenantPolicy) {
			return errors.New("Get cross-tenant deveria ser recusado")
		}
		pol, _ := domain.NewOrgMFAPolicy(fx.orgB, domain.AAL3)
		if e := NewOrgMFAPolicyStore(ttx).Set(ctx, pol); !errors.Is(e, ErrCrossTenantPolicy) {
			return errors.New("Set cross-tenant deveria ser recusado")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
}
