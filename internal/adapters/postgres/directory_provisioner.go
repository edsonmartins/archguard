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
	"github.com/jackc/pgx/v5/pgxpool"
)

// DirectoryProvisioner turns a provisioning record (from SCIM or the LDAP
// connector) into an identity + membership, DEDUPLICATING by email_hash (pacote
// 009, T-009 / RFC-0002 §6): a known e-mail NEVER creates a second identity — it
// only gains a membership in the target organization. This is the single
// reconciliation point both inbound sources share, so "the same person" is one
// identity across every tenant (RFC-0007 success criterion 1).
//
// No plaintext e-mail is stored or compared — only its deployment-keyed HMAC
// (email_hash) through the KeyCustodian (INV-7). No password is imported.
type DirectoryProvisioner struct {
	pool      *pgxpool.Pool
	custodian domain.KeyCustodian
}

// NewDirectoryProvisioner builds the provisioner over the pool and the key
// custodian (for the e-mail HMAC).
func NewDirectoryProvisioner(pool *pgxpool.Pool, custodian domain.KeyCustodian) *DirectoryProvisioner {
	return &DirectoryProvisioner{pool: pool, custodian: custodian}
}

// ProvisionUser reconciles rec into an identity + membership in orgID and returns
// the identity id (the SCIM resource id). It resolves the identity by email_hash:
// found ⇒ only the membership is ensured; not found ⇒ a new identity is created
// (with just the e-mail hash — encryption of the address is a later layer) and
// then the membership. It is idempotent: an existing membership is left as-is.
func (p *DirectoryProvisioner) ProvisionUser(ctx context.Context, orgID uuid.UUID, rec domain.DirectorySyncRecord) (string, error) {
	if rec.Email == "" {
		return "", fmt.Errorf("directory_provisioner: e-mail ausente (chave de deduplicação)")
	}
	idn, err := p.resolveOrCreateIdentity(ctx, rec.Email)
	if err != nil {
		return "", err
	}
	if err := p.ensureMembership(ctx, orgID, idn.ID); err != nil {
		return "", err
	}
	return idn.ID.String(), nil
}

// ProvisionFederated is the JIT path for a validated federated login (SAML/OIDC,
// T-012 / spec "JIT provisioning com e-mail conhecido"). It resolves the identity
// by e-mail — NEVER creating a second identity for a known e-mail — and ensures the
// membership is ACTIVE, creating it if absent or REACTIVATING it if suspended
// ("cria ou ativa o membership"). A revoked membership is not resurrected.
func (p *DirectoryProvisioner) ProvisionFederated(ctx context.Context, orgID uuid.UUID, fed domain.FederatedIdentity) (string, error) {
	if err := fed.Validate(); err != nil {
		return "", err
	}
	idn, err := p.resolveOrCreateIdentity(ctx, fed.Email)
	if err != nil {
		return "", err
	}
	if err := p.ensureActiveMembership(ctx, orgID, idn.ID); err != nil {
		return "", err
	}
	return idn.ID.String(), nil
}

// resolveOrCreateIdentity is the shared dedup core: resolve the identity by
// email_hash, or create a new one carrying only the hash. Never duplicates.
func (p *DirectoryProvisioner) resolveOrCreateIdentity(ctx context.Context, email string) (domain.Identity, error) {
	identities := NewIdentityStore(p.pool)
	idn, err := identities.FindByEmail(ctx, p.custodian, email)
	switch {
	case err == nil:
		return idn, nil
	case errors.Is(err, ErrIdentityNotFound):
		return p.createIdentity(ctx, identities, email)
	default:
		return domain.Identity{}, fmt.Errorf("directory_provisioner: resolução por email_hash falhou: %w", err)
	}
}

// createIdentity mints a human identity carrying only the e-mail hash (dedup key).
// On a concurrent create that loses the unique-index race, it re-resolves the
// winner so two simultaneous provisions still converge to ONE identity.
func (p *DirectoryProvisioner) createIdentity(ctx context.Context, identities *IdentityStore, email string) (domain.Identity, error) {
	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		return domain.Identity{}, err
	}
	hash, err := p.custodian.HashEmail(email)
	if err != nil {
		return domain.Identity{}, err
	}
	idn.EmailHash = hash
	if err := identities.Create(ctx, idn); err != nil {
		// Lost the race against a concurrent provision of the same e-mail — the
		// winner's row now exists; resolve it instead of duplicating.
		if existing, ferr := identities.FindByEmail(ctx, p.custodian, email); ferr == nil {
			return existing, nil
		}
		return domain.Identity{}, fmt.Errorf("directory_provisioner: criação de identidade falhou: %w", err)
	}
	return idn, nil
}

// ensureMembership creates the identity's membership in orgID if absent. An
// existing (active/suspended/invited) membership is idempotent success; a
// previously REVOKED membership is not silently resurrected by a directory sync
// (ErrPreviouslyRevoked surfaces).
func (p *DirectoryProvisioner) ensureMembership(ctx context.Context, orgID, identityID uuid.UUID) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	m, err := domain.NewMembership(identityID, orgID)
	if err != nil {
		return err
	}
	err = NewTenantRepository(p.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		cerr := NewTenantMembershipStore(ttx).Create(ctx, m)
		if errors.Is(cerr, ErrAlreadyMember) {
			return nil // idempotent: the membership already exists
		}
		return cerr
	})
	if err != nil {
		return fmt.Errorf("directory_provisioner: garantia de membership falhou: %w", err)
	}
	return nil
}

// ensureActiveMembership guarantees the identity has an ACTIVE membership in orgID
// (the JIT semantic): absent ⇒ create; suspended ⇒ reactivate; active ⇒ no-op;
// invited ⇒ activate; revoked ⇒ not resurrected (ErrPreviouslyRevoked).
func (p *DirectoryProvisioner) ensureActiveMembership(ctx context.Context, orgID, identityID uuid.UUID) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	err = NewTenantRepository(p.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		ms := NewTenantMembershipStore(ttx)
		m, ferr := ms.FindByIdentity(ctx, identityID)
		if errors.Is(ferr, ErrMembershipNotFound) {
			nm, nerr := domain.NewMembership(identityID, orgID)
			if nerr != nil {
				return nerr
			}
			return ms.Create(ctx, nm)
		}
		if ferr != nil {
			return ferr
		}
		switch m.Status {
		case domain.MembershipActive:
			return nil
		case domain.MembershipSuspended:
			if rerr := m.Resume(); rerr != nil {
				return rerr
			}
			return ms.SaveReactivation(ctx, m)
		case domain.MembershipInvited:
			if aerr := m.Activate(); aerr != nil {
				return aerr
			}
			return ms.SaveActivation(ctx, m)
		default: // revoked (terminal)
			return ErrPreviouslyRevoked
		}
	})
	if err != nil {
		return fmt.Errorf("directory_provisioner: garantia de membership ativo falhou: %w", err)
	}
	return nil
}
