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

package boot

import (
	"context"
	"errors"
	"fmt"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedBuiltInAdmin ensures the inherited built-in admin has a domain identity and
// an active membership in the named organization, so the login bridge can
// establish a new-model session and the console works for that admin.
//
// It is idempotent (safe to run every boot) and dedup-respecting (RFC-0002): it
// never creates a duplicate identity — an existing identity for the e-mail is
// reused, and an existing membership is left untouched. The caller runs it only
// where custody is available (dev); conformant profiles skip it.
func SeedBuiltInAdmin(ctx context.Context, pool *pgxpool.Pool, custodian domain.KeyCustodian, orgName, email string) error {
	orgID, err := organizationIDByName(ctx, pool, orgName)
	if err != nil {
		return fmt.Errorf("resolver organização %q: %w", orgName, err)
	}

	emailHash, err := custodian.HashEmail(email)
	if err != nil {
		return fmt.Errorf("hash do e-mail do admin: %w", err)
	}

	idn, _, err := provisionAdminIdentity(ctx, postgres.NewIdentityStore(pool), emailHash)
	if err != nil {
		return fmt.Errorf("identidade do admin: %w", err)
	}

	if err := ensureAdminMembership(ctx, pool, idn.ID, orgID); err != nil {
		return fmt.Errorf("membership do admin: %w", err)
	}
	return nil
}

// identityProvisioner is the subset of the identity store the seed needs — a small
// interface so the dedup decision is unit-testable without a database.
type identityProvisioner interface {
	FindByEmailHash(ctx context.Context, emailHash []byte) (domain.Identity, error)
	Create(ctx context.Context, idn domain.Identity) error
}

// provisionAdminIdentity returns the admin's domain identity, creating it only when
// no identity already matches the e-mail hash (dedup, RFC-0002 — never a
// duplicate). created reports whether a new identity was written.
func provisionAdminIdentity(ctx context.Context, store identityProvisioner, emailHash []byte) (domain.Identity, bool, error) {
	idn, err := store.FindByEmailHash(ctx, emailHash)
	switch {
	case err == nil:
		return idn, false, nil // reuse the existing identity — no duplicate
	case errors.Is(err, postgres.ErrIdentityNotFound):
		idn, err = domain.NewIdentity(domain.IdentityHuman)
		if err != nil {
			return domain.Identity{}, false, err
		}
		idn.EmailHash = emailHash
		if err := store.Create(ctx, idn); err != nil {
			return domain.Identity{}, false, err
		}
		return idn, true, nil
	default:
		return domain.Identity{}, false, err
	}
}

// organizationIDByName resolves the stable UUID (migração 0003) of the legacy
// organization row with the given name — the value the membership references.
func organizationIDByName(ctx context.Context, pool *pgxpool.Pool, name string) (uuid.UUID, error) {
	var idText string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM organization WHERE name = $1", name).Scan(&idText); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(idText)
}

// ensureAdminMembership creates an active membership for the identity in the org,
// tolerating an already-existing one (idempotent). A previously-revoked membership
// is surfaced rather than silently readmitted (R3, RFC-0002).
func ensureAdminMembership(ctx context.Context, pool *pgxpool.Pool, identityID, orgID uuid.UUID) error {
	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return err
	}
	m, err := domain.NewMembership(identityID, orgID)
	if err != nil {
		return err
	}
	return postgres.NewTenantRepository(pool, scope).WithTenantTx(ctx, func(ttx *postgres.TenantTx) error {
		err := postgres.NewTenantMembershipStore(ttx).Create(ctx, m)
		if errors.Is(err, postgres.ErrAlreadyMember) {
			return nil // idempotent: the admin is already a member
		}
		return err
	})
}
