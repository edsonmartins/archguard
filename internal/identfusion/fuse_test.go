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

package identfusion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/credmigration"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/internal/identdedup"
	"github.com/google/uuid"
)

// mapResolver resolves legacy org names from a map.
type mapResolver map[string]uuid.UUID

func (m mapResolver) OrganizationID(_ context.Context, owner string) (uuid.UUID, error) {
	id, ok := m[owner]
	if !ok {
		return uuid.Nil, fmt.Errorf("organização %q desconhecida", owner)
	}
	return id, nil
}

// memVault is a test double of the vault.
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

func (v *memVault) Delete(_ context.Context, ref string) error {
	delete(v.secrets, ref)
	return nil
}

// fusablePlan builds a valid two-account plan (org-a primary with pwd+totp,
// org-b with pwd+webauthn).
func fusablePlan() (Plan, mapResolver) {
	group := identdedup.Group{
		EmailHashHex: "00112233445566778899aabbccddeeff",
		Accounts: []identdedup.AccountRef{
			{Owner: "org-a", Name: "alice", HasPassword: true, HasTOTP: true},
			{Owner: "org-b", Name: "alice.b", HasPassword: true, HasWebAuthn: true},
		},
	}
	plan := Plan{
		Group: group,
		Approval: Approval{
			ApprovedBy:        "seguranca@integralltech",
			GroupEmailHashHex: group.EmailHashHex,
			Primary:           AccountKey{Owner: "org-a", Name: "alice"},
		},
		Credentials: map[AccountKey]credmigration.LegacyCredentials{
			{Owner: "org-a", Name: "alice"}: {
				PasswordHash: "hashA", PasswordType: "pbkdf2-salt", PasswordSalt: "s",
				TotpSecret: "SEED-A", RecoveryCodes: []string{"code-1"},
			},
			{Owner: "org-b", Name: "alice.b"}: {
				PasswordHash: "hashB", PasswordType: "pbkdf2-salt", PasswordSalt: "s",
				WebAuthn: [][]byte{[]byte("pubkey-b")},
			},
		},
	}
	return plan, mapResolver{"org-a": uuid.New(), "org-b": uuid.New()}
}

// A regra central: NÃO existe fusão sem aprovação humana explícita, e a
// aprovação autoriza exatamente UM grupo.
func TestFuseRequiresBoundHumanApproval(t *testing.T) {
	ctx := context.Background()
	vault := &memVault{}

	// Sem aprovador.
	plan, orgs := fusablePlan()
	plan.Approval.ApprovedBy = ""
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrFusionNotApproved) {
		t.Fatalf("sem aprovador: err = %v, quero ErrFusionNotApproved", err)
	}

	// Aprovação de OUTRO grupo (hash diferente) não autoriza este.
	plan, orgs = fusablePlan()
	plan.Approval.GroupEmailHashHex = "ffff0000ffff0000ffff0000ffff0000"
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrApprovalMismatch) {
		t.Fatalf("hash divergente: err = %v, quero ErrApprovalMismatch", err)
	}

	// Conta primária fora do grupo.
	plan, orgs = fusablePlan()
	plan.Approval.Primary = AccountKey{Owner: "org-z", Name: "intrusa"}
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrApprovalMismatch) {
		t.Fatalf("primária fora do grupo: err = %v, quero ErrApprovalMismatch", err)
	}
}

// Defesa em profundidade: as classes de conflito do T-015 são revalidadas —
// grupo conflitado nunca executa, mesmo que alguém o aprove.
func TestFuseRefusesNonFusableGroups(t *testing.T) {
	ctx := context.Background()
	vault := &memVault{}

	// Uma conta só.
	plan, orgs := fusablePlan()
	plan.Group.Accounts = plan.Group.Accounts[:1]
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrGroupNotFusable) {
		t.Fatalf("1 conta: err = %v, quero ErrGroupNotFusable", err)
	}

	// Duplicata na mesma organização (violaria R3).
	plan, orgs = fusablePlan()
	plan.Group.Accounts[1].Owner = "org-a"
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrGroupNotFusable) {
		t.Fatalf("mesma org: err = %v, quero ErrGroupNotFusable", err)
	}

	// Conta de serviço no grupo.
	plan, orgs = fusablePlan()
	plan.Group.Accounts[1].IsServiceAccount = true
	if _, err := Fuse(ctx, plan, orgs, vault); !errors.Is(err, ErrGroupNotFusable) {
		t.Fatalf("conta de serviço: err = %v, quero ErrGroupNotFusable", err)
	}
}

func TestFuseBuildsIdentityMembershipsAndCredentials(t *testing.T) {
	ctx := context.Background()
	vault := &memVault{}
	plan, orgs := fusablePlan()

	res, err := Fuse(ctx, plan, orgs, vault)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}

	// Identidade única, humana, carregando o hash do grupo (o e-mail nunca é
	// necessário em claro).
	if res.Identity.Type != domain.IdentityHuman || res.Identity.Status != domain.IdentityActive {
		t.Fatalf("identidade fundida errada: %+v", res.Identity)
	}
	if fmt.Sprintf("%x", res.Identity.EmailHash) != plan.Group.EmailHashHex {
		t.Fatalf("email_hash não corresponde ao grupo")
	}

	// Um membership ATIVO por organização, todos da identidade fundida.
	if len(res.Memberships) != 2 {
		t.Fatalf("memberships: %d, quero 2", len(res.Memberships))
	}
	gotOrgs := map[uuid.UUID]bool{}
	for _, m := range res.Memberships {
		if m.IdentityID != res.Identity.ID || m.Status != domain.MembershipActive {
			t.Fatalf("membership errado: %+v", m)
		}
		gotOrgs[m.OrganizationID] = true
	}
	if len(gotOrgs) != 2 {
		t.Fatalf("memberships não cobrem as 2 organizações")
	}

	// Credenciais: senha e TOTP da primária, recovery code da primária e
	// WebAuthn da secundária — a identidade fundida NÃO perde tipo de fator.
	types := map[domain.FactorType]int{}
	for _, c := range res.Credentials {
		if c.IdentityID != res.Identity.ID {
			t.Fatalf("credencial de outra identidade: %+v", c)
		}
		types[c.Type]++
	}
	want := map[domain.FactorType]int{
		domain.FactorPassword: 1, domain.FactorTOTP: 1,
		domain.FactorRecoveryCode: 1, domain.FactorWebAuthn: 1,
	}
	for ft, n := range want {
		if types[ft] != n {
			t.Fatalf("fator %s: %d, quero %d (tipos: %+v)", ft, types[ft], n, types)
		}
	}
	// O seed TOTP foi ao cofre, não à credencial.
	if len(vault.secrets) != 1 {
		t.Fatalf("cofre deveria ter 1 segredo (seed TOTP), tem %d", len(vault.secrets))
	}

	// Nada descartado em silêncio: a senha da secundária está no relatório.
	joined := strings.Join(res.DroppedFactors, "\n")
	if !strings.Contains(joined, "senha de org-b/alice.b") {
		t.Fatalf("descartes não reportados: %v", res.DroppedFactors)
	}
	if res.ForcePasswordReset {
		t.Fatalf("senha primária hasheada não deveria forçar reset")
	}
}

// Senha da primária em claro propaga o reset forçado do credmigration
// (INV-1/INV-7: plaintext jamais é carregado).
func TestFusePropagatesForcedReset(t *testing.T) {
	ctx := context.Background()
	plan, orgs := fusablePlan()
	primary := AccountKey{Owner: "org-a", Name: "alice"}
	lc := plan.Credentials[primary]
	lc.PasswordType = "plain"
	plan.Credentials[primary] = lc

	res, err := Fuse(ctx, plan, orgs, &memVault{})
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if !res.ForcePasswordReset {
		t.Fatalf("senha primária em claro deveria forçar reset")
	}
	for _, c := range res.Credentials {
		if c.Type == domain.FactorPassword {
			t.Fatalf("senha em claro não pode virar credencial")
		}
	}
}

func TestFuseFailsOnUnresolvedOrganization(t *testing.T) {
	ctx := context.Background()
	plan, _ := fusablePlan()
	if _, err := Fuse(ctx, plan, mapResolver{"org-a": uuid.New()}, &memVault{}); err == nil {
		t.Fatalf("organização não resolvida deveria falhar a fusão")
	}
}
