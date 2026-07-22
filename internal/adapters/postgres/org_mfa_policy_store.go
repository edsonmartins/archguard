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
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrCrossTenantPolicy is returned when a tenant-scoped policy store is asked to
// read or write a policy of a different organization than its scope.
var ErrCrossTenantPolicy = errors.New("postgres: política de MFA de outra organização recusada")

// OrgMFAPolicyStore is the tenant-scoped store for organization_mfa_policy. Built
// on a TenantTx, it carries the explicit organization_id predicate (Barreira 1)
// and the SET LOCAL tenant setting the RLS policy reads (Barreira 2). It is the
// admin path for reading and setting one organization's MFA floor.
type OrgMFAPolicyStore struct {
	ttx *TenantTx
}

// NewOrgMFAPolicyStore builds the store on an open tenant transaction.
func NewOrgMFAPolicyStore(ttx *TenantTx) *OrgMFAPolicyStore {
	return &OrgMFAPolicyStore{ttx: ttx}
}

// Get returns the organization's effective policy: the stored one, or the
// platform baseline (DefaultOrgMFAPolicy) when no row is set — an unset policy is
// the baseline decision, not an error. It refuses an organization other than the
// store's scope (ErrCrossTenantPolicy). A query FAILURE is returned as an error
// (fail-closed): the caller must never treat an unreadable policy as the default.
func (s *OrgMFAPolicyStore) Get(ctx context.Context, organizationID uuid.UUID) (domain.OrgMFAPolicy, error) {
	if organizationID != s.ttx.scope.OrganizationID() {
		return domain.OrgMFAPolicy{}, fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantPolicy, organizationID, s.ttx.scope.OrganizationID())
	}
	const q = `SELECT minimum_aal FROM organization_mfa_policy WHERE organization_id = $1`
	var minAAL string
	err := s.ttx.tx.QueryRow(ctx, q, organizationID.String()).Scan(&minAAL)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultOrgMFAPolicy(organizationID), nil
	}
	if err != nil {
		return domain.OrgMFAPolicy{}, fmt.Errorf("postgres: leitura de política de MFA falhou: %w", err)
	}
	return domain.NewOrgMFAPolicy(organizationID, domain.AAL(minAAL))
}

// Set upserts the organization's policy. It refuses a policy of another
// organization than the store's scope (ErrCrossTenantPolicy).
func (s *OrgMFAPolicyStore) Set(ctx context.Context, policy domain.OrgMFAPolicy) error {
	if policy.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantPolicy, policy.OrganizationID, s.ttx.scope.OrganizationID())
	}
	if !policy.MinimumAAL.Valid() {
		return fmt.Errorf("postgres: nível mínimo inválido %q", policy.MinimumAAL)
	}
	const q = `
		INSERT INTO organization_mfa_policy (organization_id, minimum_aal)
		VALUES ($1, $2)
		ON CONFLICT (organization_id)
		DO UPDATE SET minimum_aal = EXCLUDED.minimum_aal, updated_at = now()`
	if _, err := s.ttx.tx.Exec(ctx, q, policy.OrganizationID.String(), string(policy.MinimumAAL)); err != nil {
		return fmt.Errorf("postgres: gravação de política de MFA falhou: %w", err)
	}
	return nil
}

// OrgPolicyAuthority implements domain.TenantAuthPolicy over the pool: for an
// organization it opens a tenant-scoped read and returns the org's minimum AAL
// (the baseline AAL1 when unset). It replaces the provisional
// tenantswitch.ProfilePolicy — the real per-organization policy the tenant switch
// (T-011) and the assurance guard consult. Fail-closed: a store failure is
// propagated so the caller denies, never a permissive default.
type OrgPolicyAuthority struct {
	pool *pgxpool.Pool
}

// NewOrgPolicyAuthority builds the authority over the connection pool.
func NewOrgPolicyAuthority(pool *pgxpool.Pool) *OrgPolicyAuthority {
	return &OrgPolicyAuthority{pool: pool}
}

// RequiredAAL returns the minimum assurance the organization demands. It reads
// the policy under a tenant context pinned to that organization (so RLS admits
// exactly that row), defaulting to the platform baseline when none is set.
func (a *OrgPolicyAuthority) RequiredAAL(ctx context.Context, organizationID uuid.UUID) (domain.AAL, error) {
	scope, err := domain.NewTenantScope(organizationID)
	if err != nil {
		return "", fmt.Errorf("postgres: escopo de organização inválido: %w", err)
	}
	var policy domain.OrgMFAPolicy
	err = NewTenantRepository(a.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		var e error
		policy, e = NewOrgMFAPolicyStore(ttx).Get(ctx, organizationID)
		return e
	})
	if err != nil {
		return "", err
	}
	return policy.MinimumAAL, nil
}
