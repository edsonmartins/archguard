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
)

// Errors of the invite flow.
var (
	// ErrUnknownInviteEmail is returned when the invited e-mail matches no
	// existing identity. T-013 covers LINKING an existing identity to a new
	// organization (spec: "Vinculação a nova organização"); creating a brand-new
	// identity from an invite (first-access onboarding, per-holder encryption)
	// is out of scope here (architect's decision, 2026-07-21; that flow arrives
	// with packages 008/009). Nothing is created silently.
	ErrUnknownInviteEmail = errors.New("postgres: e-mail convidado não corresponde a identidade existente — criação de identidade está fora do escopo do convite")
	// ErrIdentityNotInvitable is returned when the identity exists but is not
	// active (suspended or deprovisioned): a retired or blocked identity does
	// not gain new tenants (fail-closed).
	ErrIdentityNotInvitable = errors.New("postgres: identidade não está ativa — convite recusado")
	// ErrNotInviteOwner is returned when an acceptance is attempted by an
	// identity other than the invited one — only the invited person accepts.
	ErrNotInviteOwner = errors.New("postgres: aceite recusado — o membership não pertence à identidade")
)

// Inviter orchestrates the invite-and-link flow of T-013 (ADR-0006: an
// existing identity is linked to a new organization by an accepted invitation,
// NEVER by silently creating a duplicate account). It operates in the scope of
// the inviting tenant: lookup by e-mail HASH (plaintext never touches the
// database), link the EXISTING identity with an invited membership, and later
// activate it on acceptance — each operation one tenant-pinned transaction.
type Inviter struct {
	repo      *TenantRepository
	custodian domain.KeyCustodian
	audit     AuditEmitter
}

// NewInviter wires the tenant-scoped repository with the key custodian that
// hashes invited e-mails and an optional audit emitter (nil ⇒ not
// instrumented). When set, invite and accept record membership.invite /
// membership.accept events atomically in their transaction (T-017).
func NewInviter(repo *TenantRepository, custodian domain.KeyCustodian, audit AuditEmitter) *Inviter {
	return &Inviter{repo: repo, custodian: custodian, audit: audit}
}

// InviteByEmail links the existing identity holding email to the repository's
// tenant, as an INVITED membership (grants nothing until accepted). It is the
// spec scenario made code: the identity is found by e-mail hash and gains a
// NEW MEMBERSHIP — a new identity is never created (ErrUnknownInviteEmail
// refuses instead). R3 collisions are classified by the store
// (ErrAlreadyMember / ErrPreviouslyRevoked); a non-active identity is refused
// (ErrIdentityNotInvitable). invitedBy records who issued the invitation.
func (i *Inviter) InviteByEmail(ctx context.Context, email string, invitedBy uuid.UUID) (domain.Membership, error) {
	var out domain.Membership
	err := i.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		idn, err := NewIdentityStore(ttx.Tx()).FindByEmail(ctx, i.custodian, email)
		if errors.Is(err, ErrIdentityNotFound) {
			return ErrUnknownInviteEmail
		}
		if err != nil {
			return err
		}
		if idn.Status != domain.IdentityActive {
			return fmt.Errorf("%w: status %s", ErrIdentityNotInvitable, idn.Status)
		}

		m, err := domain.NewInvitedMembership(idn.ID, i.repo.Scope().OrganizationID(), invitedBy)
		if err != nil {
			return err
		}
		if err := NewTenantMembershipStore(ttx).Create(ctx, m); err != nil {
			return err
		}
		if err := emitAudit(ctx, ttx.Tx(), i.audit, m.OrganizationID, domain.ActionMembershipInvite,
			domain.AuditTarget{Type: "identity", ID: idn.Subject, Label: "convite"},
			"convite de identidade existente para o tenant"); err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return out, nil
}

// Accept activates an invited membership of the repository's tenant, on behalf
// of identityID — the invited person, and only them (ErrNotInviteOwner). The
// domain transition (invited → active) runs first; the store persists it only
// while the row is still invited (ErrNotInvited on a concurrent change).
func (i *Inviter) Accept(ctx context.Context, membershipID, identityID uuid.UUID) (domain.Membership, error) {
	var out domain.Membership
	err := i.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		store := NewTenantMembershipStore(ttx)
		m, err := store.Get(ctx, membershipID)
		if err != nil {
			return err
		}
		if m.IdentityID != identityID {
			return fmt.Errorf("%w: membership %s", ErrNotInviteOwner, m.ID)
		}
		// Fail-closed: an identity that is no longer active (suspended or
		// deprovisioned) does not gain a new tenant by accepting an invite. The
		// suspension cascade leaves invited memberships untouched (they grant
		// nothing), so without this check a suspended identity could activate a
		// pending invite. Mirror the invite-side ErrIdentityNotInvitable guard.
		idn, err := NewIdentityStore(ttx.Tx()).Get(ctx, identityID)
		if err != nil {
			return err
		}
		if idn.Status != domain.IdentityActive {
			return fmt.Errorf("%w: status %s", ErrIdentityNotInvitable, idn.Status)
		}
		if err := m.Activate(); err != nil {
			return err
		}
		if err := store.SaveActivation(ctx, m); err != nil {
			return err
		}
		if err := emitAudit(ctx, ttx.Tx(), i.audit, m.OrganizationID, domain.ActionMembershipAccept,
			domain.AuditTarget{Type: "identity", ID: idn.Subject, Label: "aceite de convite"},
			"aceite de convite: membership ativado"); err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return out, nil
}
