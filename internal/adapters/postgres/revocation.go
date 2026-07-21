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
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// IdentityMembershipStore is the identity-axis view of membership: the rows of
// ONE identity across ALL its organizations — the membership leg of the
// identity-level cascade (RFC-0002 R4). Built on an IdentityTx, so it carries
// the explicit identity_id predicate (Barreira 1) and the SET LOCAL identity
// setting the amended RLS policy reads (Barreira 2, migration 0014). It only
// writes lifecycle transitions in bulk; creating memberships stays with the
// tenant-scoped store.
type IdentityMembershipStore struct {
	itx *IdentityTx
}

// NewIdentityMembershipStore builds the store on an open identity transaction.
func NewIdentityMembershipStore(itx *IdentityTx) *IdentityMembershipStore {
	return &IdentityMembershipStore{itx: itx}
}

// SuspendAllActive suspends every ACTIVE membership of the identity — the
// recoverable cascade of identity suspension (decisão do arquiteto,
// 2026-07-21: suspensão da identidade suspende vínculos, não os revoga;
// terminal é reservado ao deprovisionamento). Invited, suspended and revoked
// rows are untouched. Returns how many memberships moved.
func (s *IdentityMembershipStore) SuspendAllActive(ctx context.Context) (int, error) {
	const q = `
		UPDATE membership
		SET status = $2, updated_at = now()
		WHERE identity_id = $1 AND status = $3`
	tag, err := s.itx.tx.Exec(ctx, q, s.itx.scope.IdentityID().String(),
		string(domain.MembershipSuspended), string(domain.MembershipActive))
	if err != nil {
		return 0, fmt.Errorf("postgres: suspensão em cascata de memberships falhou: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RevokeAllNonRevoked revokes every non-revoked membership of the identity —
// the terminal cascade of deprovisioning (R4). Rows stay for the audit trail.
// Returns how many memberships moved. Idempotent.
func (s *IdentityMembershipStore) RevokeAllNonRevoked(ctx context.Context) (int, error) {
	const q = `
		UPDATE membership
		SET status = $2, revoked_at = COALESCE(revoked_at, now()), updated_at = now()
		WHERE identity_id = $1 AND status <> $2`
	tag, err := s.itx.tx.Exec(ctx, q, s.itx.scope.IdentityID().String(),
		string(domain.MembershipRevoked))
	if err != nil {
		return 0, fmt.Errorf("postgres: revogação em cascata de memberships falhou: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// MembershipRevoker orchestrates the MEMBERSHIP revocation cascade (spec
// "Revogação de membership"): in ONE tenant-pinned transaction, the membership
// becomes revoked (terminal, R4) and the identity's active sessions IN THIS
// TENANT are ended. Memberships and sessions in other organizations are out of
// this transaction's reach by construction — which is exactly the spec's AND
// clause.
type MembershipRevoker struct {
	repo *TenantRepository
}

// NewMembershipRevoker builds the revoker on a tenant-scoped repository.
func NewMembershipRevoker(repo *TenantRepository) *MembershipRevoker {
	return &MembershipRevoker{repo: repo}
}

// RevokeMembership revokes one membership of the repository's tenant and ends
// its sessions. Idempotent end-to-end. Returns the revoked membership and how
// many sessions were ended.
func (r *MembershipRevoker) RevokeMembership(ctx context.Context, membershipID uuid.UUID) (domain.Membership, int, error) {
	var (
		out      domain.Membership
		sessions int
	)
	err := r.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		memberships := NewTenantMembershipStore(ttx)
		m, err := memberships.Get(ctx, membershipID)
		if err != nil {
			return err
		}
		m.Revoke() // terminal and idempotent in the domain
		if err := memberships.SaveRevocation(ctx, m); err != nil {
			return err
		}
		if sessions, err = NewTenantSessionStore(ttx).RevokeByMembership(ctx, m.ID); err != nil {
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

// CascadeReport summarizes one identity-level cascade.
type CascadeReport struct {
	Identity         domain.Identity
	MembershipsMoved int
	SessionsRevoked  int
}

// IdentityLifecycle orchestrates the IDENTITY-level cascades in ONE
// identity-pinned transaction each (RFC-0002 §5 — a partial cascade is an
// inconsistent state a security product cannot leave behind):
//
//   - Suspend: identity → suspended; active memberships → suspended
//     (recoverable); ALL sessions → revoked (a session is always terminal —
//     whoever returns authenticates again). Scenario "Suspensão da identidade".
//   - Deprovision: identity → deprovisioned (terminal, R5); every non-revoked
//     membership → revoked; ALL sessions → revoked. RFC-0002 R4 in full.
type IdentityLifecycle struct {
	repo *IdentityRepository
}

// NewIdentityLifecycle builds the lifecycle orchestrator on an identity-scoped
// repository.
func NewIdentityLifecycle(repo *IdentityRepository) *IdentityLifecycle {
	return &IdentityLifecycle{repo: repo}
}

// Suspend blocks the identity and cascades: memberships suspended
// (recoverable), sessions revoked. A deprovisioned identity refuses (terminal).
func (l *IdentityLifecycle) Suspend(ctx context.Context) (CascadeReport, error) {
	return l.cascade(ctx, func(idn *domain.Identity) error { return idn.Suspend() },
		func(ctx context.Context, ms *IdentityMembershipStore) (int, error) {
			return ms.SuspendAllActive(ctx)
		})
}

// Deprovision retires the identity permanently (R5) and cascades terminally:
// memberships revoked, sessions revoked (R4).
func (l *IdentityLifecycle) Deprovision(ctx context.Context) (CascadeReport, error) {
	return l.cascade(ctx, func(idn *domain.Identity) error { idn.Deprovision(); return nil },
		func(ctx context.Context, ms *IdentityMembershipStore) (int, error) {
			return ms.RevokeAllNonRevoked(ctx)
		})
}

// cascade runs one identity-level transition end to end: load → domain
// transition → persist status → memberships leg → sessions leg, one
// transaction.
func (l *IdentityLifecycle) cascade(ctx context.Context,
	transition func(*domain.Identity) error,
	memberships func(context.Context, *IdentityMembershipStore) (int, error),
) (CascadeReport, error) {
	var report CascadeReport
	err := l.repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
		identities := NewIdentityStore(itx.Tx())
		idn, err := identities.Get(ctx, l.repo.Scope().IdentityID())
		if err != nil {
			return err
		}
		if err := transition(&idn); err != nil {
			return err
		}
		if err := identities.SaveStatus(ctx, idn); err != nil {
			return err
		}
		if report.MembershipsMoved, err = memberships(ctx, NewIdentityMembershipStore(itx)); err != nil {
			return err
		}
		if report.SessionsRevoked, err = NewIdentitySessionStore(itx).RevokeAll(ctx); err != nil {
			return err
		}
		report.Identity = idn
		return nil
	})
	if err != nil {
		return CascadeReport{}, err
	}
	return report, nil
}
