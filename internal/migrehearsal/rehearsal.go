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

// Package migrehearsal is the migration rehearsal of the identity model
// (RFC-0002 §6, T-019): it runs the WHOLE pipeline — legacy extraction →
// inventory/dedup (identdedup, T-015) → approved fusion (identfusion, T-016) →
// credential migration (credmigration, T-005) → role re-pointing
// (rolemigration, T-006) — against a copy of the production database, and
// produces the result report the package gate demands: NO MFA factor lost and
// NO duplicated human identity.
//
// It is meant for a disposable COPY (RFC-0002 §6: migration is irreversible in
// practice — verified backup and rehearsal are prerequisites of the real run).
// Groups without human approval are NOT fused — they are reported as pending
// (silent automatic fusion is forbidden); conflicted groups are reported and
// skipped. The rehearsal fails validation loudly instead of hiding a loss.
package migrehearsal

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/credmigration"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/identdedup"
	"github.com/casdoor/casdoor/internal/identfusion"
	"github.com/casdoor/casdoor/internal/rolemigration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Approvals maps a fusion group's e-mail hash (hex) to its human approval.
// A candidate group absent from this map is NOT fused — it goes to the
// report's PendingApproval list.
type Approvals map[string]identfusion.Approval

// Report is the rehearsal outcome — the artifact the operator reviews before
// authorizing the real migration.
type Report struct {
	IdentitiesCreated  int
	MembershipsCreated int
	CredentialsByType  map[domain.FactorType]int
	// MembershipsPerOrg counts created memberships by legacy organization name
	// (the per-tenant validation of RFC-0002 §6 step 6).
	MembershipsPerOrg map[string]int
	// ForcedResets lists accounts whose legacy plaintext password was refused
	// (INV-1/INV-7): the identity must set a new password on first login.
	ForcedResets []string
	// DroppedFactors lists non-primary single-slot factors superseded in
	// fusions — explicit, never silent (identfusion).
	DroppedFactors []string
	// PendingApproval lists fusion groups (hash prefix) awaiting human
	// approval. They were NOT migrated.
	PendingApproval []string
	// Conflicts are the T-015 findings needing human review. Not migrated.
	Conflicts []identdedup.Conflict
	// FactorLoss lists identities that would LOSE an authentication factor: a
	// TOTP/WebAuthn factor present in some source account but absent after
	// migration, or a password that vanished without a forced reset (leaving no
	// login path). The gate requires this EMPTY.
	FactorLoss []string
	// PreExisting lists accounts skipped because an identity with their e-mail
	// hash already existed (a prior rehearsal run on the same copy, or an
	// identity created by the invite flow). Skipping makes Run idempotent and
	// avoids both a mid-run unique-violation abort and an orphaned vault write.
	PreExisting []string
	// DuplicateHumanEmails counts email_hash values shared by more than one
	// human identity after the run. The DB's unique partial index on
	// identity(email_hash) already makes this impossible, so this is a
	// belt-and-suspenders cross-check, not the primary enforcement. The gate
	// requires ZERO.
	DuplicateHumanEmails int
	RoleAssignments      int
	RolesUnresolved      []string
}

// Validate enforces the package gate on the report: no MFA factor loss and no
// duplicated human identity.
func (r Report) Validate() error {
	if len(r.FactorLoss) > 0 {
		return fmt.Errorf("migrehearsal: %d identidade(s) perderiam fator MFA: %s",
			len(r.FactorLoss), strings.Join(r.FactorLoss, "; "))
	}
	if r.DuplicateHumanEmails > 0 {
		return fmt.Errorf("migrehearsal: %d e-mail(s) com identidade humana duplicada", r.DuplicateHumanEmails)
	}
	return nil
}

// account couples the inventory view of a legacy account with its credential
// material.
type account struct {
	ref   identdedup.LegacyAccount
	creds credmigration.LegacyCredentials
}

// Run executes the rehearsal over the database behind pool. Every persisted
// group runs in ONE transaction (RFC-0002 §5); vault writes (TOTP seeds)
// happen before the transaction opens (RFC-0004 §4).
func Run(ctx context.Context, pool *pgxpool.Pool, custodian domain.KeyCustodian, secrets domain.SecretStore, approvals Approvals) (Report, error) {
	report := Report{
		CredentialsByType: map[domain.FactorType]int{},
		MembershipsPerOrg: map[string]int{},
	}

	accounts, err := extractLegacyAccounts(ctx, pool)
	if err != nil {
		return report, err
	}
	refs := make([]identdedup.LegacyAccount, 0, len(accounts))
	byKey := map[identfusion.AccountKey]account{}
	for _, a := range accounts {
		refs = append(refs, a.ref)
		byKey[identfusion.AccountKey{Owner: a.ref.Owner, Name: a.ref.Name}] = a
	}
	inventory, err := identdedup.BuildInventory(refs, custodian)
	if err != nil {
		return report, err
	}
	report.Conflicts = inventory.Conflicts

	orgs := &orgResolver{pool: pool, cache: map[string]uuid.UUID{}}
	// membershipByOrgUser feeds the role re-pointing resolver ("org/user").
	membershipByOrgUser := map[string]rolemigration.ResolvedMembership{}

	persist := func(idn domain.Identity, memberships []domain.Membership, creds []domain.Credential, orgNames []string, userNames []string) error {
		scope, err := domain.NewIdentityScope(idn.ID)
		if err != nil {
			return err
		}
		repo := postgres.NewIdentityRepository(pool, scope)
		if err := repo.WithIdentityTx(ctx, func(itx *postgres.IdentityTx) error {
			if err := postgres.NewIdentityStore(itx.Tx()).Create(ctx, idn); err != nil {
				return err
			}
			for _, m := range memberships {
				// One fused identity may span several organizations in this single
				// transaction, which no single tenant scope could write. Since
				// migration 0015 restricts membership INSERT to the tenant axis
				// (no self-grant under identity context), the rehearsal — a
				// migration operation — runs as a privileged migration role that
				// bypasses RLS, like every other step of the data migration.
				if _, err := itx.Tx().Exec(ctx,
					"INSERT INTO membership (id, identity_id, organization_id, status) VALUES ($1, $2, $3, $4)",
					m.ID.String(), m.IdentityID.String(), m.OrganizationID.String(), string(m.Status)); err != nil {
					return fmt.Errorf("migrehearsal: membership: %w", err)
				}
			}
			credStore := postgres.NewCredentialStore(itx.Tx())
			for _, c := range creds {
				if err := credStore.Create(ctx, c); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		report.IdentitiesCreated++
		report.MembershipsCreated += len(memberships)
		for _, c := range creds {
			report.CredentialsByType[c.Type]++
		}
		for i, m := range memberships {
			report.MembershipsPerOrg[orgNames[i]]++
			membershipByOrgUser[orgNames[i]+"/"+userNames[i]] = rolemigration.ResolvedMembership{
				MembershipID: m.ID, OrganizationID: m.OrganizationID,
			}
		}
		return nil
	}

	// 1:1 accounts and e-mail-less accounts: one identity + one membership each.
	for _, g := range inventory.Singles {
		exists, err := identityExistsByHash(ctx, pool, mustHash(g.EmailHashHex))
		if err != nil {
			return report, err
		}
		if exists {
			report.PreExisting = append(report.PreExisting, g.Accounts[0].Owner+"/"+g.Accounts[0].Name)
			continue
		}
		if err := migrateSingle(ctx, g.Accounts[0], mustHash(g.EmailHashHex), byKey, orgs, secrets, persist, &report); err != nil {
			return report, err
		}
	}
	for _, ref := range inventory.NoEmail {
		if err := migrateSingle(ctx, ref, nil, byKey, orgs, secrets, persist, &report); err != nil {
			return report, err
		}
	}

	// Fusion candidates: only with explicit human approval.
	for _, g := range inventory.FusionCandidates {
		approval, ok := approvals[g.EmailHashHex]
		if !ok {
			report.PendingApproval = append(report.PendingApproval, identdedup.HashPrefix(g.EmailHashHex))
			continue
		}
		exists, err := identityExistsByHash(ctx, pool, mustHash(g.EmailHashHex))
		if err != nil {
			return report, err
		}
		if exists {
			report.PreExisting = append(report.PreExisting, identdedup.HashPrefix(g.EmailHashHex)+" (grupo fundido)")
			continue
		}
		plan := identfusion.Plan{Group: g, Approval: approval,
			Credentials: map[identfusion.AccountKey]credmigration.LegacyCredentials{}}
		for _, ref := range g.Accounts {
			key := identfusion.AccountKey{Owner: ref.Owner, Name: ref.Name}
			plan.Credentials[key] = byKey[key].creds
		}
		fused, err := identfusion.Fuse(ctx, plan, orgs, secrets)
		if err != nil {
			return report, err
		}
		orgNames := make([]string, len(g.Accounts))
		userNames := make([]string, len(g.Accounts))
		for i, ref := range g.Accounts {
			orgNames[i], userNames[i] = ref.Owner, ref.Name
		}
		if err := persist(fused.Identity, fused.Memberships, fused.Credentials, orgNames, userNames); err != nil {
			cleanupVault(ctx, secrets, fused.Credentials)
			return report, err
		}
		report.DroppedFactors = append(report.DroppedFactors, fused.DroppedFactors...)
		if fused.ForcePasswordReset {
			report.ForcedResets = append(report.ForcedResets,
				fmt.Sprintf("%s/%s (grupo fundido)", approval.Primary.Owner, approval.Primary.Name))
		}
		checkFactorPreservation(g.Accounts, fused.Credentials, fused.ForcePasswordReset, identdedup.HashPrefix(g.EmailHashHex), &report)
	}

	// Roles: re-point Role.Users[] to memberships (T-006), per organization.
	if err := migrateRoles(ctx, pool, membershipByOrgUser, orgs, &report); err != nil {
		return report, err
	}

	// Belt-and-suspenders: the DB's unique partial index already forbids two
	// human identities sharing an email_hash, so this can only ever read 0 — it
	// is a cross-check that the index is present and honored, not the primary
	// guard (the pre-existing skip above is what keeps the run idempotent).
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT email_hash FROM identity
			WHERE email_hash IS NOT NULL AND type = 'human'
			GROUP BY email_hash HAVING count(*) > 1
		) dup`).Scan(&report.DuplicateHumanEmails); err != nil {
		return report, fmt.Errorf("migrehearsal: verificação de duplicatas falhou: %w", err)
	}

	sort.Strings(report.PendingApproval)
	sort.Strings(report.FactorLoss)
	return report, nil
}

// migrateSingle migrates one stand-alone account (unique e-mail or no e-mail).
func migrateSingle(ctx context.Context, ref identdedup.AccountRef, emailHash []byte,
	byKey map[identfusion.AccountKey]account, orgs *orgResolver, secrets domain.SecretStore,
	persist func(domain.Identity, []domain.Membership, []domain.Credential, []string, []string) error,
	report *Report,
) error {
	kind := domain.IdentityHuman
	if ref.IsServiceAccount {
		kind = domain.IdentityService
	}
	idn, err := domain.NewIdentity(kind)
	if err != nil {
		return err
	}
	idn.EmailHash = emailHash

	orgID, err := orgs.OrganizationID(ctx, ref.Owner)
	if err != nil {
		return err
	}
	m, err := domain.NewMembership(idn.ID, orgID)
	if err != nil {
		return err
	}

	src := byKey[identfusion.AccountKey{Owner: ref.Owner, Name: ref.Name}]
	migrated, err := credmigration.Migrate(ctx, idn.ID, src.creds, secrets)
	if err != nil {
		return fmt.Errorf("migrehearsal: credenciais de %s/%s: %w", ref.Owner, ref.Name, err)
	}
	if migrated.ForcePasswordReset {
		report.ForcedResets = append(report.ForcedResets, ref.Owner+"/"+ref.Name)
	}
	if err := persist(idn, []domain.Membership{m}, migrated.Credentials,
		[]string{ref.Owner}, []string{ref.Name}); err != nil {
		// The vault Put in credmigration.Migrate happened before the persistence
		// transaction (RFC-0004 §4); if persistence failed, the seed is orphaned
		// — compensate by deleting it so no vaulted secret outlives its row.
		cleanupVault(ctx, secrets, migrated.Credentials)
		return err
	}
	checkFactorPreservation([]identdedup.AccountRef{ref}, migrated.Credentials, migrated.ForcePasswordReset, ref.Owner+"/"+ref.Name, report)
	return nil
}

// cleanupVault best-effort deletes the vault references of the given
// credentials — compensation for a vault write whose database transaction
// failed. Errors are ignored: this runs on an already-failing path and must
// not mask the original error.
func cleanupVault(ctx context.Context, secrets domain.SecretStore, creds []domain.Credential) {
	for _, c := range creds {
		if c.SecretRef != "" {
			_ = secrets.Delete(ctx, c.SecretRef)
		}
	}
}

// checkFactorPreservation compares the authentication factors present in the
// source accounts against the migrated credentials: a factor present before and
// absent after is a LOSS — the finding that fails the gate. A password is not
// "lost" when a reset was intentionally forced (INV-1/INV-7): the reset flow
// still gives the identity a login path. But a password that vanishes with NO
// surviving password credential AND NO forced reset leaves the identity without
// a knowledge factor — a silent loss the gate must catch.
func checkFactorPreservation(sources []identdedup.AccountRef, creds []domain.Credential, passwordReset bool, label string, report *Report) {
	has := map[domain.FactorType]bool{}
	for _, c := range creds {
		has[c.Type] = true
	}
	wantPassword, wantTOTP, wantWebAuthn := false, false, false
	for _, s := range sources {
		wantPassword = wantPassword || s.HasPassword
		wantTOTP = wantTOTP || s.HasTOTP
		wantWebAuthn = wantWebAuthn || s.HasWebAuthn
	}
	if wantPassword && !has[domain.FactorPassword] && !passwordReset {
		report.FactorLoss = append(report.FactorLoss, label+": senha presente na origem, ausente após a migração e sem reset forçado — identidade sem caminho de login")
	}
	if wantTOTP && !has[domain.FactorTOTP] {
		report.FactorLoss = append(report.FactorLoss, label+": TOTP presente na origem e ausente após a migração")
	}
	if wantWebAuthn && !has[domain.FactorWebAuthn] {
		report.FactorLoss = append(report.FactorLoss, label+": WebAuthn presente na origem e ausente após a migração")
	}
}

// migrateRoles re-points every legacy role's Users[] to memberships and
// persists the assignments per organization (one tenant transaction each).
func migrateRoles(ctx context.Context, pool *pgxpool.Pool,
	memberships map[string]rolemigration.ResolvedMembership, orgs *orgResolver, report *Report,
) error {
	rows, err := pool.Query(ctx, `SELECT owner, name, id::text, COALESCE(users, '') FROM role WHERE id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("migrehearsal: leitura de papéis falhou: %w", err)
	}
	defer rows.Close()

	type legacyRole struct {
		owner, name string
		id          uuid.UUID
		users       []string
	}
	var roles []legacyRole
	for rows.Next() {
		var r legacyRole
		var idText, usersJSON string
		if err := rows.Scan(&r.owner, &r.name, &idText, &usersJSON); err != nil {
			return fmt.Errorf("migrehearsal: leitura de papel falhou: %w", err)
		}
		if r.id, err = uuid.Parse(idText); err != nil {
			return fmt.Errorf("migrehearsal: id de papel inválido %q: %w", idText, err)
		}
		if strings.HasPrefix(strings.TrimSpace(usersJSON), "[") {
			if err := json.Unmarshal([]byte(usersJSON), &r.users); err != nil {
				return fmt.Errorf("migrehearsal: users do papel %s/%s inválido: %w", r.owner, r.name, err)
			}
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrehearsal: iteração de papéis falhou: %w", err)
	}

	resolver := mapMembershipResolver(memberships)
	for _, r := range roles {
		res, err := rolemigration.Migrate(ctx, r.id, r.users, resolver)
		if err != nil {
			return err
		}
		report.RolesUnresolved = append(report.RolesUnresolved, res.Unresolved...)
		byOrg := map[uuid.UUID][]domain.RoleAssignment{}
		for _, ra := range res.Assignments {
			byOrg[ra.OrganizationID] = append(byOrg[ra.OrganizationID], ra)
		}
		for orgID, ras := range byOrg {
			scope, err := domain.NewTenantScope(orgID)
			if err != nil {
				return err
			}
			repo := postgres.NewTenantRepository(pool, scope)
			if err := repo.WithTenantTx(ctx, func(ttx *postgres.TenantTx) error {
				store := postgres.NewRoleAssignmentStore(ttx)
				for _, ra := range ras {
					if err := store.Create(ctx, ra); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
			report.RoleAssignments += len(ras)
		}
	}
	return nil
}

// extractLegacyAccounts reads the legacy user table with SQL primitives (no
// XORM). Plaintext e-mail stays in memory only long enough to be hashed by the
// inventory; TOTP/recovery material flows straight into credmigration, which
// removes it from the clear.
func extractLegacyAccounts(ctx context.Context, pool *pgxpool.Pool) ([]account, error) {
	rows, err := pool.Query(ctx, `
		SELECT owner, name, COALESCE(email, ''), COALESCE(type, ''),
		       COALESCE(password, ''), COALESCE(password_salt, ''), COALESCE(password_type, ''),
		       COALESCE(totp_secret, ''), COALESCE(recovery_codes, ''), webauthn_credentials
		FROM "user"`)
	if err != nil {
		return nil, fmt.Errorf("migrehearsal: extração do legado falhou: %w", err)
	}
	defer rows.Close()

	var out []account
	for rows.Next() {
		var (
			a                                     account
			email, legacyType                     string
			password, salt, ptype, totp, recovery string
			webauthnRaw                           []byte
		)
		if err := rows.Scan(&a.ref.Owner, &a.ref.Name, &email, &legacyType,
			&password, &salt, &ptype, &totp, &recovery, &webauthnRaw); err != nil {
			return nil, fmt.Errorf("migrehearsal: leitura de usuário legado falhou: %w", err)
		}
		a.ref.Email = email
		a.ref.IsServiceAccount = legacyType == "bot" || legacyType == "service" || legacyType == "service-account"
		a.creds = credmigration.LegacyCredentials{
			PasswordHash: password, PasswordSalt: salt, PasswordType: ptype, TotpSecret: totp,
		}
		if strings.HasPrefix(strings.TrimSpace(recovery), "[") {
			if err := json.Unmarshal([]byte(recovery), &a.creds.RecoveryCodes); err != nil {
				return nil, fmt.Errorf("migrehearsal: recovery_codes de %s/%s inválido: %w", a.ref.Owner, a.ref.Name, err)
			}
		}
		if len(webauthnRaw) > 0 {
			var items []json.RawMessage
			if err := json.Unmarshal(webauthnRaw, &items); err != nil {
				return nil, fmt.Errorf("migrehearsal: webauthn_credentials de %s/%s inválido: %w", a.ref.Owner, a.ref.Name, err)
			}
			for _, it := range items {
				a.creds.WebAuthn = append(a.creds.WebAuthn, []byte(it))
			}
		}
		a.ref.HasPassword = password != ""
		a.ref.HasTOTP = totp != ""
		a.ref.HasWebAuthn = len(a.creds.WebAuthn) > 0
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrehearsal: iteração do legado falhou: %w", err)
	}
	return out, nil
}

// identityExistsByHash reports whether an identity already carries this e-mail
// hash. A nil/empty hash (a no-e-mail account) is never "already there" by
// hash, so it returns false — those accounts are minted fresh each run. The
// pre-check keeps the rehearsal idempotent: a re-run, or an identity created by
// the invite flow, is skipped instead of aborting the whole run on the unique
// index (and it avoids the vault write that would otherwise be orphaned by that
// abort).
func identityExistsByHash(ctx context.Context, pool *pgxpool.Pool, emailHash []byte) (bool, error) {
	if len(emailHash) == 0 {
		return false, nil
	}
	var exists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM identity WHERE email_hash = $1)", emailHash).Scan(&exists); err != nil {
		return false, fmt.Errorf("migrehearsal: verificação de identidade existente falhou: %w", err)
	}
	return exists, nil
}

// orgResolver resolves legacy organization names to stable UUIDs (0003) via
// pgx — the real implementation of identfusion.OrganizationResolver.
type orgResolver struct {
	pool  *pgxpool.Pool
	cache map[string]uuid.UUID
}

func (r *orgResolver) OrganizationID(ctx context.Context, owner string) (uuid.UUID, error) {
	if id, ok := r.cache[owner]; ok {
		return id, nil
	}
	var idText string
	if err := r.pool.QueryRow(ctx,
		"SELECT id::text FROM organization WHERE name = $1", owner).Scan(&idText); err != nil {
		return uuid.Nil, fmt.Errorf("migrehearsal: organização %q não encontrada: %w", owner, err)
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return uuid.Nil, fmt.Errorf("migrehearsal: id da organização %q inválido: %w", owner, err)
	}
	r.cache[owner] = id
	return id, nil
}

// mapMembershipResolver adapts the created-membership map to rolemigration.
type mapMembershipResolver map[string]rolemigration.ResolvedMembership

func (m mapMembershipResolver) Resolve(_ context.Context, orgUser string) (rolemigration.ResolvedMembership, bool, error) {
	rm, ok := m[orgUser]
	return rm, ok, nil
}

// mustHash decodes an inventory hash (hex produced by identdedup — always
// valid).
func mustHash(hexHash string) []byte {
	b, err := hex.DecodeString(hexHash)
	if err != nil {
		return nil
	}
	return b
}

// Render writes the rehearsal report (pt-BR, no plaintext personal data).
func (r Report) Render(w io.Writer) error {
	p := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}
	if err := p("# Relatório do ensaio de migração (T-019)\n\n"); err != nil {
		return err
	}
	if err := p("Identidades criadas: %d · Memberships: %d · Papéis vinculados: %d\n",
		r.IdentitiesCreated, r.MembershipsCreated, r.RoleAssignments); err != nil {
		return err
	}
	for org, n := range r.MembershipsPerOrg {
		if err := p("  - %s: %d membership(s)\n", org, n); err != nil {
			return err
		}
	}
	if err := p("Credenciais por tipo: %v\n\n", r.CredentialsByType); err != nil {
		return err
	}
	if err := p("Resets forçados (senha em claro no legado, INV-1/INV-7): %d %v\n",
		len(r.ForcedResets), r.ForcedResets); err != nil {
		return err
	}
	if err := p("Fatores de slot único não carregados (fusões): %d %v\n",
		len(r.DroppedFactors), r.DroppedFactors); err != nil {
		return err
	}
	if err := p("Fusões AGUARDANDO aprovação humana (não migradas): %d %v\n",
		len(r.PendingApproval), r.PendingApproval); err != nil {
		return err
	}
	if err := p("Conflitos para revisão humana (não migrados): %d\n", len(r.Conflicts)); err != nil {
		return err
	}
	if err := p("Papéis com usuários não resolvidos: %d %v\n\n",
		len(r.RolesUnresolved), r.RolesUnresolved); err != nil {
		return err
	}
	if err := p("VALIDAÇÃO DO GATE — perda de fator MFA: %d · identidades humanas duplicadas: %d\n",
		len(r.FactorLoss), r.DuplicateHumanEmails); err != nil {
		return err
	}
	for _, loss := range r.FactorLoss {
		if err := p("  PERDA: %s\n", loss); err != nil {
			return err
		}
	}
	return nil
}
