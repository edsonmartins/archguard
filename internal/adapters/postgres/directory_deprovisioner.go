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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// DirectoryDeprovisioner reflects a directory deactivation onto ArchGuard (pacote
// 009, T-005 / spec "Desprovisionamento reflete o diretório"): when the source
// directory deactivates a user, the corresponding membership is SUSPENDED (never
// deleted — history is preserved) and the tenant's sessions for it are ended, in
// ONE tenant-pinned transaction. It mirrors MembershipRevoker, but suspension is
// RECOVERABLE: a re-activation in the directory can resume the membership.
type DirectoryDeprovisioner struct {
	repo  *TenantRepository
	audit AuditEmitter
}

// NewDirectoryDeprovisioner builds the deprovisioner on a tenant-scoped repository,
// with an optional audit emitter (nil ⇒ not instrumented).
func NewDirectoryDeprovisioner(repo *TenantRepository, audit AuditEmitter) *DirectoryDeprovisioner {
	return &DirectoryDeprovisioner{repo: repo, audit: audit}
}

// SuspendForDeactivation suspends the membership and ends its tenant sessions when
// the directory deactivated the user. It is IDEMPOTENT and a no-op when the
// membership is not active (already suspended/revoked, or invited): those return
// the membership unchanged with zero sessions ended. Returns the membership and
// the count of sessions ended.
func (d *DirectoryDeprovisioner) SuspendForDeactivation(ctx context.Context, membershipID uuid.UUID) (domain.Membership, int, error) {
	var (
		out      domain.Membership
		sessions int
	)
	err := d.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		memberships := NewTenantMembershipStore(ttx)
		m, err := memberships.Get(ctx, membershipID)
		if err != nil {
			return err
		}
		out = m
		// Idempotent: only an ACTIVE membership is suspended. Suspending an already
		// suspended/revoked one is a no-op (no session churn, no audit noise).
		if m.Status != domain.MembershipActive {
			return nil
		}
		if err := m.Suspend(); err != nil {
			return err
		}
		if err := memberships.SaveSuspension(ctx, m); err != nil {
			return err
		}
		if sessions, err = NewTenantSessionStore(ttx).RevokeByMembership(ctx, m.ID); err != nil {
			return err
		}
		if err := emitAudit(ctx, ttx.Tx(), d.audit, m.OrganizationID, domain.ActionMembershipSuspend,
			domain.AuditTarget{Type: "membership", ID: m.ID.String(), Label: "suspensão por desativação no diretório"},
			"desativação no diretório: suspensão do membership e encerramento das sessões do tenant"); err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return domain.Membership{}, 0, err
	}
	return out, sessions, nil
}
