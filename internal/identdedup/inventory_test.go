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

package identdedup

import (
	"reflect"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/domain"
)

func testCustodian(t *testing.T) domain.KeyCustodian {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cust, err := keycustodian.NewProvisional(key)
	if err != nil {
		t.Fatalf("custodian: %v", err)
	}
	return cust
}

func TestBuildInventoryClassifies(t *testing.T) {
	cust := testCustodian(t)
	accounts := []LegacyAccount{
		// Mesma pessoa em duas orgs (case-insensitive) → candidata a fusão.
		{Owner: "org-a", Name: "alice", Email: "Alice@Example.com", HasPassword: true, HasTOTP: true},
		{Owner: "org-b", Name: "alice.b", Email: "alice@example.com", HasPassword: true, HasWebAuthn: true},
		// E-mail único → 1:1.
		{Owner: "org-a", Name: "bob", Email: "bob@example.com", HasPassword: true},
		// Sem e-mail (conta de serviço) → identidade própria, sem hash.
		{Owner: "org-a", Name: "ci-bot", IsServiceAccount: true},
		// Mesmo e-mail DUAS VEZES na MESMA org → conflito R3.
		{Owner: "org-c", Name: "carol", Email: "carol@example.com", HasPassword: true},
		{Owner: "org-c", Name: "carol.old", Email: "carol@example.com"},
		// E-mail compartilhado por humana e serviço → conflito de natureza.
		{Owner: "org-a", Name: "dana", Email: "dana@example.com", HasPassword: true},
		{Owner: "org-b", Name: "dana-svc", Email: "dana@example.com", IsServiceAccount: true},
		// E-mail inhasheável (só espaços) → conflito de dado de origem.
		{Owner: "org-a", Name: "erro", Email: "   "},
	}

	inv, err := BuildInventory(accounts, cust)
	if err != nil {
		t.Fatalf("BuildInventory: %v", err)
	}

	if len(inv.Singles) != 1 || inv.Singles[0].Accounts[0].Name != "bob" {
		t.Fatalf("singles: %+v, quero só bob", inv.Singles)
	}
	if len(inv.FusionCandidates) != 1 {
		t.Fatalf("candidatas: %d, quero 1 (alice)", len(inv.FusionCandidates))
	}
	fusion := inv.FusionCandidates[0]
	if len(fusion.Accounts) != 2 || fusion.Accounts[0].Owner != "org-a" || fusion.Accounts[1].Owner != "org-b" {
		t.Fatalf("grupo de fusão errado (normalização case-insensitive falhou?): %+v", fusion.Accounts)
	}
	// Os fatores de cada conta viajam no relatório — insumo do "sem perda de
	// fator MFA" do T-019.
	if !fusion.Accounts[0].HasTOTP || !fusion.Accounts[1].HasWebAuthn {
		t.Fatalf("fatores perdidos no inventário: %+v", fusion.Accounts)
	}
	if len(inv.NoEmail) != 1 || inv.NoEmail[0].Name != "ci-bot" {
		t.Fatalf("no-email: %+v, quero ci-bot", inv.NoEmail)
	}

	kinds := map[ConflictKind]int{}
	for _, c := range inv.Conflicts {
		kinds[c.Kind]++
	}
	if kinds[ConflictSameOrgDuplicate] != 1 || kinds[ConflictMixedTypes] != 1 || kinds[ConflictUnhashableEmail] != 1 {
		t.Fatalf("conflitos: %+v, quero 1 de cada tipo", kinds)
	}

	// Grupo conflitado NUNCA aparece também como proposta.
	for _, g := range append(inv.Singles, inv.FusionCandidates...) {
		for _, r := range g.Accounts {
			if r.Owner == "org-c" || strings.HasPrefix(r.Name, "dana") {
				t.Fatalf("conta conflitada vazou para proposta: %+v", r)
			}
		}
	}
}

// A ferramenta é determinística: a mesma base em QUALQUER ordem produz o mesmo
// inventário (o ensaio do T-019 diffa execuções).
func TestBuildInventoryDeterministic(t *testing.T) {
	cust := testCustodian(t)
	accounts := []LegacyAccount{
		{Owner: "org-b", Name: "z", Email: "z@example.com"},
		{Owner: "org-a", Name: "alice", Email: "alice@example.com", HasTOTP: true},
		{Owner: "org-c", Name: "alice.c", Email: "ALICE@example.com"},
		{Owner: "org-a", Name: "ci", IsServiceAccount: true},
	}
	first, err := BuildInventory(accounts, cust)
	if err != nil {
		t.Fatalf("BuildInventory: %v", err)
	}
	reversed := make([]LegacyAccount, 0, len(accounts))
	for i := len(accounts) - 1; i >= 0; i-- {
		reversed = append(reversed, accounts[i])
	}
	second, err := BuildInventory(reversed, cust)
	if err != nil {
		t.Fatalf("BuildInventory invertido: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("inventário não determinístico:\n%+v\n!=\n%+v", first, second)
	}
}

// O relatório nunca expõe e-mail em claro: contas são referenciadas por
// owner/name e grupos por prefixo do hash (minimização, I-3.x).
func TestRenderContainsNoPlaintextEmail(t *testing.T) {
	cust := testCustodian(t)
	accounts := []LegacyAccount{
		{Owner: "org-a", Name: "alice", Email: "alice.secret@example.com", HasTOTP: true},
		{Owner: "org-b", Name: "alice.b", Email: "alice.secret@example.com", HasPassword: true},
		{Owner: "org-c", Name: "carol", Email: "carol.secret@example.com"},
		{Owner: "org-c", Name: "carol2", Email: "carol.secret@example.com"},
	}
	inv, err := BuildInventory(accounts, cust)
	if err != nil {
		t.Fatalf("BuildInventory: %v", err)
	}

	var sb strings.Builder
	if err := inv.Render(&sb); err != nil {
		t.Fatalf("Render: %v", err)
	}
	report := sb.String()

	for _, leak := range []string{"alice.secret", "carol.secret", "@example.com"} {
		if strings.Contains(report, leak) {
			t.Fatalf("relatório vazou e-mail em claro (%q):\n%s", leak, report)
		}
	}
	// Mas referencia as contas e os achados.
	for _, want := range []string{"org-a/alice", "org-b/alice.b", "same_org_duplicate", "totp", "CONFLITOS: 1"} {
		if !strings.Contains(report, want) {
			t.Fatalf("relatório sem %q:\n%s", want, report)
		}
	}
}

func TestBuildInventoryRequiresCustodian(t *testing.T) {
	if _, err := BuildInventory(nil, nil); err == nil {
		t.Fatalf("sem custodiante deveria falhar")
	}
}
