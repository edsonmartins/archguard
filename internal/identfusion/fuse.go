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

// Package identfusion is the ASSISTED fusion routine of the identity migration
// (RFC-0002 §6 step 2, T-016): it turns ONE approved deduplication group
// (T-015) into one global identity + one membership per organization,
// migrating credentials through credmigration (INV-7 preserved).
//
// "Fusão automática silenciosa é proibida" (design 002 §"Migração") is
// STRUCTURAL here: Fuse does not run without an Approval naming a human
// approver, bound to the exact group being fused (by e-mail hash) and choosing
// which account's single-slot factors prevail. An approval of group X can
// never authorize group Y.
//
// Like the other migration mechanisms (credmigration, rolemigration), this is
// a testable function over legacy primitives; persistence and mass execution
// belong to the rehearsal (T-019).
package identfusion

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/casdoor/casdoor/internal/credmigration"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/identdedup"
	"github.com/google/uuid"
)

// Errors of the assisted fusion.
var (
	// ErrFusionNotApproved is returned when the approval is missing or does not
	// name a human approver — the mandatory-human-approval rule in code.
	ErrFusionNotApproved = errors.New("identfusion: fusão exige aprovação humana explícita")
	// ErrApprovalMismatch is returned when the approval does not match the group
	// (wrong hash, primary account not in the group): an approval authorizes
	// exactly one fusion, never another.
	ErrApprovalMismatch = errors.New("identfusion: aprovação não corresponde ao grupo a fundir")
	// ErrGroupNotFusable is returned when the group is not a valid fusion
	// candidate (fewer than two accounts, same-org duplicate, service account in
	// the mix) — these are the T-015 conflict classes, revalidated here as
	// defense in depth.
	ErrGroupNotFusable = errors.New("identfusion: grupo não é fundível — deveria ter sido barrado como conflito no inventário")
)

// AccountKey locates one legacy account (owner, name).
type AccountKey struct {
	Owner string
	Name  string
}

// Approval is the explicit human approval a fusion requires. It binds the
// approver to ONE group (by e-mail hash) and elects the PRIMARY account, whose
// single-slot factors (password, TOTP, recovery codes) prevail — the schema
// allows one password and one TOTP per identity, so when several accounts
// carry them a human, not the tool, decides which survives.
type Approval struct {
	// ApprovedBy identifies the human who approved this fusion. Required.
	ApprovedBy string
	// GroupEmailHashHex must equal the group's hash — the approval is bound to
	// this exact fusion.
	GroupEmailHashHex string
	// Primary elects the account whose password/TOTP/recovery codes prevail.
	// Must be one of the group's accounts.
	Primary AccountKey
}

// Plan is one approved fusion to execute.
type Plan struct {
	// Group is the T-015 fusion candidate being executed.
	Group identdedup.Group
	// Approval is the mandatory human approval.
	Approval Approval
	// Credentials carries each account's legacy credential material, keyed by
	// account. Missing entries mean the account has no material to migrate.
	Credentials map[AccountKey]credmigration.LegacyCredentials
}

// Result is the fused output, ready for the persistence step of the rehearsal
// (T-019): one identity, its memberships (via the organization resolver) and
// the INV-7-well-formed credentials.
type Result struct {
	Identity    domain.Identity
	Memberships []domain.Membership
	Credentials []domain.Credential
	// ForcePasswordReset propagates credmigration's refusal to carry a
	// plaintext legacy password (INV-1/INV-7).
	ForcePasswordReset bool
	// DroppedFactors lists, explicitly, every non-primary single-slot factor
	// that was NOT carried (superseded by the primary's). Nothing is dropped
	// silently: this list goes into the migration report, and the identity
	// always retains at least the primary's factors — factor TYPES are never
	// lost (gate do pacote).
	DroppedFactors []string
}

// OrganizationResolver resolves a legacy organization name to its stable UUID
// (organization.id, migration 0003). The real implementation reads via pgx;
// tests use a map. Mirrors rolemigration's MembershipResolver seam.
type OrganizationResolver interface {
	OrganizationID(ctx context.Context, owner string) (uuid.UUID, error)
}

// Fuse executes one approved fusion. It validates the approval against the
// group (human named, hash bound, primary elected within the group) and the
// group's fusability (≥2 accounts, distinct organizations, humans only),
// mints the fused identity carrying the group's e-mail hash, one ACTIVE
// membership per organization, and migrates credentials: the primary account
// contributes everything (via credmigration — TOTP seed goes to the vault,
// plaintext password forces reset); non-primary accounts contribute ONLY their
// WebAuthn credentials (multi-slot, public material); their single-slot
// factors are reported in DroppedFactors, never dropped silently.
func Fuse(ctx context.Context, plan Plan, orgs OrganizationResolver, secrets domain.SecretStore) (Result, error) {
	if orgs == nil || secrets == nil {
		return Result{}, fmt.Errorf("identfusion: resolvedor de organizações e cofre são obrigatórios")
	}
	if err := validateApproval(plan); err != nil {
		return Result{}, err
	}
	if err := validateGroup(plan.Group); err != nil {
		return Result{}, err
	}

	emailHash, err := hex.DecodeString(plan.Group.EmailHashHex)
	if err != nil || len(emailHash) == 0 {
		return Result{}, fmt.Errorf("identfusion: email_hash do grupo inválido: %v", err)
	}

	idn, err := domain.NewIdentity(domain.IdentityHuman)
	if err != nil {
		return Result{}, fmt.Errorf("identfusion: criação da identidade fundida falhou: %w", err)
	}
	idn.EmailHash = emailHash

	var res Result
	for _, ref := range plan.Group.Accounts {
		orgID, err := orgs.OrganizationID(ctx, ref.Owner)
		if err != nil {
			return Result{}, fmt.Errorf("identfusion: organização %q não resolvida: %w", ref.Owner, err)
		}
		m, err := domain.NewMembership(idn.ID, orgID)
		if err != nil {
			return Result{}, fmt.Errorf("identfusion: membership em %q: %w", ref.Owner, err)
		}
		res.Memberships = append(res.Memberships, m)

		key := AccountKey{Owner: ref.Owner, Name: ref.Name}
		lc := plan.Credentials[key]
		if key == plan.Approval.Primary {
			migrated, err := credmigration.Migrate(ctx, idn.ID, lc, secrets)
			if err != nil {
				return Result{}, fmt.Errorf("identfusion: credenciais da conta primária %s/%s: %w", key.Owner, key.Name, err)
			}
			res.Credentials = append(res.Credentials, migrated.Credentials...)
			res.ForcePasswordReset = migrated.ForcePasswordReset
			continue
		}

		// Non-primary: only multi-slot WebAuthn credentials are carried; every
		// single-slot factor left behind is reported, never silently dropped.
		webauthnOnly := credmigration.LegacyCredentials{WebAuthn: lc.WebAuthn}
		migrated, err := credmigration.Migrate(ctx, idn.ID, webauthnOnly, secrets)
		if err != nil {
			return Result{}, fmt.Errorf("identfusion: WebAuthn da conta %s/%s: %w", key.Owner, key.Name, err)
		}
		res.Credentials = append(res.Credentials, migrated.Credentials...)
		res.DroppedFactors = append(res.DroppedFactors, droppedOf(key, lc)...)
	}

	res.Identity = idn
	return res, nil
}

// validateApproval enforces the mandatory human approval, bound to this group.
func validateApproval(plan Plan) error {
	a := plan.Approval
	if a.ApprovedBy == "" {
		return ErrFusionNotApproved
	}
	if a.GroupEmailHashHex != plan.Group.EmailHashHex {
		return fmt.Errorf("%w: aprovação para hash %q, grupo é %q",
			ErrApprovalMismatch, a.GroupEmailHashHex, plan.Group.EmailHashHex)
	}
	for _, ref := range plan.Group.Accounts {
		if ref.Owner == a.Primary.Owner && ref.Name == a.Primary.Name {
			return nil
		}
	}
	return fmt.Errorf("%w: conta primária %s/%s não está no grupo",
		ErrApprovalMismatch, a.Primary.Owner, a.Primary.Name)
}

// validateGroup revalidates the T-015 conflict classes — defense in depth: a
// conflicted group must never have reached execution.
func validateGroup(g identdedup.Group) error {
	if len(g.Accounts) < 2 {
		return fmt.Errorf("%w: %d conta(s) — fusão pressupõe duplicidade", ErrGroupNotFusable, len(g.Accounts))
	}
	seen := map[string]bool{}
	for _, ref := range g.Accounts {
		if ref.IsServiceAccount {
			return fmt.Errorf("%w: %s/%s é conta de serviço (mixed_types)", ErrGroupNotFusable, ref.Owner, ref.Name)
		}
		if seen[ref.Owner] {
			return fmt.Errorf("%w: organização %q duplicada (same_org_duplicate, R3)", ErrGroupNotFusable, ref.Owner)
		}
		seen[ref.Owner] = true
	}
	return nil
}

// droppedOf lists the single-slot factors of a non-primary account.
func droppedOf(key AccountKey, lc credmigration.LegacyCredentials) []string {
	var out []string
	if lc.PasswordHash != "" {
		out = append(out, fmt.Sprintf("senha de %s/%s (prevalece a da conta primária)", key.Owner, key.Name))
	}
	if lc.TotpSecret != "" {
		out = append(out, fmt.Sprintf("TOTP de %s/%s (prevalece o da conta primária)", key.Owner, key.Name))
	}
	if len(lc.RecoveryCodes) > 0 {
		out = append(out, fmt.Sprintf("códigos de recuperação de %s/%s (prevalecem os da conta primária)", key.Owner, key.Name))
	}
	return out
}
