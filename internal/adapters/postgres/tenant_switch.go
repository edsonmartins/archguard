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

// TenantSwitcher orchestrates the tenant-switch business operation (T-012,
// design 002 §"Sessão e tenant ativo"): destination policy → domain transition
// → optimistic persist → outbox enqueue, all but the policy inside ONE
// transaction (RFC-0002 §5). The policy port is mandatory (no switch without a
// decision); the audit record is written to the transactional outbox in the
// same transaction, so it is atomic with the switch by construction.
type TenantSwitcher struct {
	repo   *IdentityRepository
	policy domain.TenantAuthPolicy
}

// NewTenantSwitcher wires the identity-scoped repository with the destination
// policy port. The audit record is written to the transactional outbox
// (migration 0016) within the switch transaction; the durable trail (pacote
// 003) drains that outbox asynchronously, outside this transaction.
func NewTenantSwitcher(repo *IdentityRepository, policy domain.TenantAuthPolicy) *TenantSwitcher {
	return &TenantSwitcher{repo: repo, policy: policy}
}

// Switch moves the session to the destination membership. Order is
// load-bearing:
//  1. the destination's AAL requirement is resolved BEFORE the transaction
//     (RFC-0004 §4: no remote call inside a database transaction) — a policy
//     error denies (INV-6, ErrDestinationPolicyUnavailable);
//  2. inside one identity-pinned transaction: the session is re-read, the
//     domain transition runs (step-up denial, foreign/inactive destination,
//     same-tenant no-op — all fail-closed), and the switch is persisted
//     optimistically (a concurrent change, or a destination membership no longer
//     active, yields ErrSwitchConflict);
//  3. the audit event is written to the transactional outbox STILL INSIDE the
//     transaction — if the enqueue fails, the transaction rolls back and the
//     switch DID NOT HAPPEN (I-5.4). An unaudited context change cannot exist,
//     and no remote call is made inside the transaction (the durable trail
//     drains the outbox later, out of band).
//
// On success the returned session carries the destination tenant and the new
// token generation — the caller derives the new token's claims from it
// (TokenContext); tokens minted before the switch carry the old generation and
// fail validation.
func (s *TenantSwitcher) Switch(ctx context.Context, sessionID uuid.UUID, dest domain.Membership) (domain.AuthSession, error) {
	required, err := s.policy.RequiredAAL(ctx, dest.OrganizationID)
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("%w: %v", domain.ErrDestinationPolicyUnavailable, err)
	}

	var out domain.AuthSession
	err = s.repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
		store := NewIdentitySessionStore(itx)
		sess, err := store.Get(ctx, sessionID)
		if err != nil {
			return err
		}
		fromMembership := sess.MembershipID
		fromGeneration := sess.TokenGeneration

		event, err := sess.SwitchTenant(dest, required)
		if err != nil {
			return err
		}
		if err := store.SaveSwitch(ctx, sess, *fromMembership, fromGeneration); err != nil {
			return err
		}
		if err := NewSessionOutbox(itx.Tx()).EnqueueTenantSwitch(ctx, event); err != nil {
			return fmt.Errorf("%w: %v", domain.ErrSwitchAuditUnavailable, err)
		}
		out = sess
		return nil
	})
	if err != nil {
		return domain.AuthSession{}, err
	}
	return out, nil
}
