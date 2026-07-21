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

package migrehearsal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/identfusion"
	"github.com/casdoor/casdoor/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// memVault captures vaulted secrets for inspection.
type memVault struct{ secrets map[string][]byte }

func (v *memVault) Put(_ context.Context, secret []byte) (string, error) {
	if v.secrets == nil {
		v.secrets = map[string][]byte{}
	}
	ref := fmt.Sprintf("ref-%d", len(v.secrets)+1)
	v.secrets[ref] = append([]byte(nil), secret...)
	return ref, nil
}

func (v *memVault) Get(_ context.Context, ref string) ([]byte, error) {
	s, ok := v.secrets[ref]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return s, nil
}

func setupRehearsal(t *testing.T) (*pgxpool.Pool, domain.KeyCustodian) {
	t.Helper()
	dsn := os.Getenv("ARCHGUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("ARCHGUARD_TEST_DSN não definido — ensaio exige PostgreSQL real")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// A cópia "de produção": tabelas legadas como o Sync2 as deixaria (o
	// subconjunto de colunas que o ensaio lê), depois as migrations.
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS organization (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
		`CREATE TABLE IF NOT EXISTS role (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name))`,
		`ALTER TABLE role ADD COLUMN IF NOT EXISTS users text`,
		`CREATE TABLE IF NOT EXISTS "user" (
			owner text NOT NULL, name text NOT NULL, PRIMARY KEY (owner, name),
			email text, type text, password text, password_salt text, password_type text,
			totp_secret text, recovery_codes text, webauthn_credentials bytea)`,
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("seed legado: %v", err)
		}
	}
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cust, err := keycustodian.NewProvisional(key)
	if err != nil {
		t.Fatalf("custodian: %v", err)
	}
	return pool, cust
}

// seedOrg creates a legacy organization and returns nothing — the rehearsal
// resolves it by name.
func seedOrg(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO organization (owner, name) VALUES ('admin', $1)", name); err != nil {
		t.Fatalf("org %s: %v", name, err)
	}
}

type legacyUserRow struct {
	owner, name, email, typ                   string
	password, salt, ptype, totp, recoveryJSON string
	webauthnJSON                              string
}

func seedUser(t *testing.T, pool *pgxpool.Pool, u legacyUserRow) {
	t.Helper()
	var webauthn []byte
	if u.webauthnJSON != "" {
		webauthn = []byte(u.webauthnJSON)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO "user" (owner, name, email, type, password, password_salt, password_type,
			totp_secret, recovery_codes, webauthn_credentials)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.owner, u.name, u.email, u.typ, u.password, u.salt, u.ptype,
		u.totp, u.recoveryJSON, webauthn); err != nil {
		t.Fatalf("user %s/%s: %v", u.owner, u.name, err)
	}
}

// cleanupOrgs removes everything the rehearsal created for the given legacy
// organizations. Identities are reached via their memberships in those orgs —
// rehearsal identities belong exclusively to the test's orgs, so nothing from
// other tests is touched.
func cleanupOrgs(t *testing.T, pool *pgxpool.Pool, orgs ...string) {
	t.Cleanup(func() {
		ctx := context.Background()
		identityIDs := map[string]bool{}
		for _, org := range orgs {
			rows, err := pool.Query(ctx, `SELECT identity_id::text FROM membership
				WHERE organization_id IN (SELECT id FROM organization WHERE name = $1)`, org)
			if err == nil {
				for rows.Next() {
					var id string
					if rows.Scan(&id) == nil {
						identityIDs[id] = true
					}
				}
				rows.Close()
			}
			_, _ = pool.Exec(ctx, `DELETE FROM role_assignment WHERE organization_id IN
				(SELECT id FROM organization WHERE name = $1)`, org)
			_, _ = pool.Exec(ctx, `DELETE FROM membership WHERE organization_id IN
				(SELECT id FROM organization WHERE name = $1)`, org)
		}
		for id := range identityIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM credential WHERE identity_id = $1`, id)
			_, _ = pool.Exec(ctx, `DELETE FROM identity WHERE id = $1`, id)
		}
		for _, org := range orgs {
			_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE owner = $1`, org)
			_, _ = pool.Exec(ctx, `DELETE FROM role WHERE owner = $1`, org)
			_, _ = pool.Exec(ctx, `DELETE FROM organization WHERE name = $1`, org)
		}
	})
}

func TestRehearsalEndToEnd(t *testing.T) {
	pool, cust := setupRehearsal(t)
	ctx := context.Background()
	org1, org2 := "reh-org1", "reh-org2"
	cleanupOrgs(t, pool, org1, org2)
	seedOrg(t, pool, org1)
	seedOrg(t, pool, org2)

	// A pessoa em dois tenants (fusão aprovada): alice.
	seedUser(t, pool, legacyUserRow{owner: org1, name: "alice", email: "alice@reh.example",
		password: "h1", salt: "s1", ptype: "pbkdf2-salt", totp: "SEED-ALICE",
		recoveryJSON: `["r1","r2"]`})
	seedUser(t, pool, legacyUserRow{owner: org2, name: "alice.ext", email: "Alice@reh.example",
		password: "h2", salt: "s2", ptype: "pbkdf2-salt",
		webauthnJSON: `[{"ID":"cred-1","PublicKey":"cHVi"}]`})
	// Senha em claro no legado: reset forçado (INV-1/INV-7).
	seedUser(t, pool, legacyUserRow{owner: org1, name: "bob", email: "bob@reh.example",
		password: "plaintext-pwd", ptype: "plain"})
	// Conta de serviço sem e-mail.
	seedUser(t, pool, legacyUserRow{owner: org1, name: "ci-bot", typ: "bot",
		password: "h3", salt: "s3", ptype: "pbkdf2-salt"})
	// Conflito: mesmo e-mail duas vezes na MESMA org (violaria R3).
	seedUser(t, pool, legacyUserRow{owner: org2, name: "carol", email: "carol@reh.example", password: "h4", ptype: "pbkdf2-salt"})
	seedUser(t, pool, legacyUserRow{owner: org2, name: "carol.old", email: "carol@reh.example"})
	// Candidata a fusão SEM aprovação: fica pendente, não migra.
	seedUser(t, pool, legacyUserRow{owner: org1, name: "dave", email: "dave@reh.example", password: "h5", ptype: "pbkdf2-salt"})
	seedUser(t, pool, legacyUserRow{owner: org2, name: "dave.ext", email: "dave@reh.example"})
	// Papel legado com um membro migrado e um fantasma.
	if _, err := pool.Exec(ctx,
		`INSERT INTO role (owner, name, users) VALUES ($1, 'admins', $2)`,
		org1, fmt.Sprintf(`["%s/alice","%s/ghost"]`, org1, org1)); err != nil {
		t.Fatalf("role: %v", err)
	}

	aliceHash, err := cust.HashEmail("alice@reh.example")
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	approvals := Approvals{
		fmt.Sprintf("%x", aliceHash): identfusion.Approval{
			ApprovedBy:        "seguranca@integralltech",
			GroupEmailHashHex: fmt.Sprintf("%x", aliceHash),
			Primary:           identfusion.AccountKey{Owner: org1, Name: "alice"},
		},
	}

	vault := &memVault{}
	report, err := Run(ctx, pool, cust, vault, approvals)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// O gate do pacote: sem perda de fator MFA, sem identidade humana duplicada.
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Contas migradas: alice (fundida), bob, ci-bot = 3 identidades; carol
	// (conflito) e dave (sem aprovação) NÃO migram.
	if report.IdentitiesCreated != 3 {
		t.Fatalf("identidades = %d, quero 3 (relatório: %+v)", report.IdentitiesCreated, report)
	}
	if report.MembershipsCreated != 4 || report.MembershipsPerOrg[org1] != 3 || report.MembershipsPerOrg[org2] != 1 {
		t.Fatalf("memberships errados: %+v", report.MembershipsPerOrg)
	}
	wantCreds := map[domain.FactorType]int{
		domain.FactorPassword: 2, domain.FactorTOTP: 1,
		domain.FactorWebAuthn: 1, domain.FactorRecoveryCode: 2,
	}
	for ft, n := range wantCreds {
		if report.CredentialsByType[ft] != n {
			t.Fatalf("credenciais %s = %d, quero %d (%+v)", ft, report.CredentialsByType[ft], n, report.CredentialsByType)
		}
	}
	if len(report.ForcedResets) != 1 || report.ForcedResets[0] != org1+"/bob" {
		t.Fatalf("resets forçados: %+v, quero só org1/bob", report.ForcedResets)
	}
	if len(report.PendingApproval) != 1 {
		t.Fatalf("pendentes de aprovação: %+v, quero 1 (dave)", report.PendingApproval)
	}
	if len(report.Conflicts) != 1 || report.Conflicts[0].Kind != "same_org_duplicate" {
		t.Fatalf("conflitos: %+v, quero same_org_duplicate de carol", report.Conflicts)
	}
	if report.RoleAssignments != 1 || len(report.RolesUnresolved) != 1 ||
		!strings.HasSuffix(report.RolesUnresolved[0], "/ghost") {
		t.Fatalf("papéis: %d vinculados, não resolvidos %+v", report.RoleAssignments, report.RolesUnresolved)
	}
	// A senha da alice.ext não prevaleceu — descartada COM registro.
	if !strings.Contains(strings.Join(report.DroppedFactors, "\n"), "senha de "+org2+"/alice.ext") {
		t.Fatalf("descartes não reportados: %+v", report.DroppedFactors)
	}

	// No banco: a alice fundida é UMA identidade com DOIS memberships.
	var aliceMemberships int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM membership WHERE identity_id =
			(SELECT id FROM identity WHERE email_hash = $1)`, aliceHash).Scan(&aliceMemberships); err != nil {
		t.Fatalf("consulta alice: %v", err)
	}
	if aliceMemberships != 2 {
		t.Fatalf("alice deveria ter 2 memberships, tem %d", aliceMemberships)
	}
	// O seed TOTP está no cofre — nunca no banco (INV-7).
	found := false
	for _, s := range vault.secrets {
		if string(s) == "SEED-ALICE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("seed TOTP da alice deveria estar no cofre")
	}
	var seedInDB int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM credential WHERE secret_ref = 'SEED-ALICE'`).Scan(&seedInDB); err != nil {
		t.Fatalf("sonda de vazamento: %v", err)
	}
	if seedInDB > 0 {
		t.Fatalf("seed TOTP vazou para o banco (INV-7)")
	}

	// Relatório sem dado pessoal em claro.
	var sb strings.Builder
	if err := report.Render(&sb); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(sb.String(), "@reh.example") {
		t.Fatalf("relatório vazou e-mail em claro:\n%s", sb.String())
	}
}

// A validação do gate ACUSA perda de fator: se a aprovação eleger uma primária
// sem TOTP quando outra conta do grupo o tem, o tipo se perderia — o relatório
// falha em vez de esconder.
func TestRehearsalDetectsFactorLoss(t *testing.T) {
	pool, cust := setupRehearsal(t)
	ctx := context.Background()
	orgX, orgY := "reh-orgx", "reh-orgy"
	cleanupOrgs(t, pool, orgX, orgY)
	seedOrg(t, pool, orgX)
	seedOrg(t, pool, orgY)

	seedUser(t, pool, legacyUserRow{owner: orgX, name: "eve", email: "eve@reh.example",
		password: "h1", ptype: "pbkdf2-salt"})
	seedUser(t, pool, legacyUserRow{owner: orgY, name: "eve.y", email: "eve@reh.example",
		totp: "SEED-EVE"})

	eveHash, err := cust.HashEmail("eve@reh.example")
	if err != nil {
		t.Fatalf("HashEmail: %v", err)
	}
	approvals := Approvals{
		fmt.Sprintf("%x", eveHash): identfusion.Approval{
			ApprovedBy:        "seguranca@integralltech",
			GroupEmailHashHex: fmt.Sprintf("%x", eveHash),
			// Primária SEM TOTP — o TOTP da eve.y seria perdido.
			Primary: identfusion.AccountKey{Owner: orgX, Name: "eve"},
		},
	}

	report, err := Run(ctx, pool, cust, &memVault{}, approvals)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.FactorLoss) == 0 {
		t.Fatalf("perda de fator TOTP deveria ter sido acusada")
	}
	if err := report.Validate(); err == nil {
		t.Fatalf("Validate deveria falhar com perda de fator")
	}
}
