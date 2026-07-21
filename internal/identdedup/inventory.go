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

// Package identdedup is the inventory-and-deduplication tool of the identity
// migration (RFC-0002 §6, steps 1–2, T-015): it groups legacy accounts by
// e-mail hash (the deployment-keyed HMAC of KeyCustodian — plaintext e-mail
// never appears in the output, only the hash and the owner/name reference) and
// classifies each group:
//
//   - one account, unique e-mail → one identity, one membership (1:1);
//   - same e-mail across DIFFERENT organizations → fusion candidate: ONE
//     identity + N memberships (the ADR-0006 model);
//   - anything ambiguous → CONFLICT, for human review. Silent automatic fusion
//     is FORBIDDEN (design 002 §"Migração") — this tool only proposes; the
//     assisted fusion (T-016) executes under explicit human approval.
//
// The tool is a pure function over legacy primitives (no XORM, no database);
// the mass extraction and execution belong to the migration rehearsal (T-019).
package identdedup

import (
	"encoding/hex"
	"fmt"
	"io"
	"sort"

	"github.com/casdoor/casdoor/internal/domain"
)

// LegacyAccount is the identifying slice of one legacy user row, extracted
// with primitives. Email arrives as stored (plaintext in the legacy schema);
// it is hashed immediately and never carried into the inventory. The factor
// flags feed the migration guarantee "no identity loses an MFA factor"
// (package gate): the reviewer sees what each account carries before a fusion.
type LegacyAccount struct {
	Owner            string // legacy organization name
	Name             string // legacy user name
	Email            string // plaintext as stored; hashed here, never output
	IsServiceAccount bool   // legacy bot/app account
	HasPassword      bool
	HasTOTP          bool
	HasWebAuthn      bool
}

// AccountRef identifies one legacy account in the inventory WITHOUT personal
// data: the (owner, name) pair locates the row for whoever executes the
// migration, and the factor flags inform the fusion review.
type AccountRef struct {
	Owner            string
	Name             string
	IsServiceAccount bool
	HasPassword      bool
	HasTOTP          bool
	HasWebAuthn      bool
}

// Group is a set of legacy accounts sharing one e-mail hash.
type Group struct {
	// EmailHashHex is the hex-encoded HMAC of the normalized e-mail — the same
	// value identity.email_hash will hold. Never the plaintext.
	EmailHashHex string
	Accounts     []AccountRef
}

// ConflictKind classifies why a group needs human review.
type ConflictKind string

const (
	// ConflictSameOrgDuplicate: the same e-mail appears more than once WITHIN
	// one organization. Fusing would demand two memberships in the same tenant —
	// impossible under R3 — so a human must decide which account prevails.
	ConflictSameOrgDuplicate ConflictKind = "same_org_duplicate"
	// ConflictMixedTypes: the e-mail is shared by human and service accounts.
	// A service account fused into a human identity (or vice versa) changes its
	// nature — never silently.
	ConflictMixedTypes ConflictKind = "mixed_types"
	// ConflictUnhashableEmail: the account has an e-mail that cannot be hashed
	// (normalizes to empty). It cannot join the dedup keyspace; a human fixes
	// the source data or confirms the account as e-mail-less.
	ConflictUnhashableEmail ConflictKind = "unhashable_email"
)

// Conflict is one finding that blocks automatic handling of its accounts.
type Conflict struct {
	Kind         ConflictKind
	EmailHashHex string // empty for unhashable e-mails
	Accounts     []AccountRef
	Detail       string
}

// Inventory is the tool's output: the full classification of the legacy
// account base. Everything under Conflicts is EXCLUDED from Singles and
// FusionCandidates — a conflicted group is never partially proposed.
type Inventory struct {
	// Singles: one account, unique e-mail → 1:1 identity + membership.
	Singles []Group
	// FusionCandidates: same e-mail in different organizations → proposal of
	// ONE identity + one membership per organization. Execution requires human
	// approval (T-016).
	FusionCandidates []Group
	// NoEmail: accounts without e-mail (typical of service accounts) → one
	// identity each, without email_hash (the partial unique index allows it).
	NoEmail []AccountRef
	// Conflicts: groups needing human review before any migration.
	Conflicts []Conflict
}

// BuildInventory hashes and groups the accounts, classifying each group. It is
// deterministic: the same input (in any order) yields the same inventory, so
// two runs over the same base can be diffed (the rehearsal of T-019 relies on
// that).
func BuildInventory(accounts []LegacyAccount, custodian domain.KeyCustodian) (Inventory, error) {
	if custodian == nil {
		return Inventory{}, fmt.Errorf("identdedup: custodiante de chaves é obrigatório")
	}

	var inv Inventory
	groups := map[string][]AccountRef{}
	for _, a := range accounts {
		ref := AccountRef{
			Owner: a.Owner, Name: a.Name, IsServiceAccount: a.IsServiceAccount,
			HasPassword: a.HasPassword, HasTOTP: a.HasTOTP, HasWebAuthn: a.HasWebAuthn,
		}
		if a.Email == "" {
			inv.NoEmail = append(inv.NoEmail, ref)
			continue
		}
		hash, err := custodian.HashEmail(a.Email)
		if err != nil {
			inv.Conflicts = append(inv.Conflicts, Conflict{
				Kind:     ConflictUnhashableEmail,
				Accounts: []AccountRef{ref},
				Detail:   fmt.Sprintf("e-mail de %s/%s não pôde ser hasheado: %v", a.Owner, a.Name, err),
			})
			continue
		}
		key := hex.EncodeToString(hash)
		groups[key] = append(groups[key], ref)
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		refs := groups[key]
		sortRefs(refs)
		g := Group{EmailHashHex: key, Accounts: refs}

		if kind, detail := classifyConflict(refs); kind != "" {
			inv.Conflicts = append(inv.Conflicts, Conflict{
				Kind: kind, EmailHashHex: key, Accounts: refs, Detail: detail,
			})
			continue
		}
		if len(refs) == 1 {
			inv.Singles = append(inv.Singles, g)
			continue
		}
		inv.FusionCandidates = append(inv.FusionCandidates, g)
	}

	sortRefs(inv.NoEmail)
	sort.SliceStable(inv.Conflicts, func(i, j int) bool {
		if inv.Conflicts[i].EmailHashHex != inv.Conflicts[j].EmailHashHex {
			return inv.Conflicts[i].EmailHashHex < inv.Conflicts[j].EmailHashHex
		}
		return inv.Conflicts[i].Detail < inv.Conflicts[j].Detail
	})
	return inv, nil
}

// classifyConflict decides whether a hash group needs human review.
func classifyConflict(refs []AccountRef) (ConflictKind, string) {
	seenOrg := map[string]string{}
	humans, services := 0, 0
	for _, r := range refs {
		if prev, dup := seenOrg[r.Owner]; dup {
			return ConflictSameOrgDuplicate,
				fmt.Sprintf("organização %q tem mais de uma conta com o mesmo e-mail (%s e %s) — fusão violaria a R3", r.Owner, prev, r.Name)
		}
		seenOrg[r.Owner] = r.Name
		if r.IsServiceAccount {
			services++
		} else {
			humans++
		}
	}
	if humans > 0 && services > 0 {
		return ConflictMixedTypes,
			fmt.Sprintf("e-mail compartilhado por %d conta(s) humana(s) e %d de serviço — natureza da identidade exige decisão humana", humans, services)
	}
	return "", ""
}

// sortRefs orders accounts by (owner, name) for deterministic output.
func sortRefs(refs []AccountRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Owner != refs[j].Owner {
			return refs[i].Owner < refs[j].Owner
		}
		return refs[i].Name < refs[j].Name
	})
}

// Render writes the human-readable conflict report (pt-BR). It contains NO
// plaintext e-mail — accounts are referenced by owner/name and groups by hash
// prefix, which is enough for the operator to locate the rows.
func (inv Inventory) Render(w io.Writer) error {
	p := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}
	factors := func(r AccountRef) string {
		out := ""
		if r.HasPassword {
			out += " senha"
		}
		if r.HasTOTP {
			out += " totp"
		}
		if r.HasWebAuthn {
			out += " webauthn"
		}
		if out == "" {
			out = " (sem fator)"
		}
		return out
	}
	hashPrefix := func(h string) string {
		if len(h) > 12 {
			return h[:12] + "…"
		}
		return h
	}

	if err := p("# Inventário de deduplicação de identidades (T-015)\n\n"); err != nil {
		return err
	}
	if err := p("Contas 1:1: %d · Grupos de fusão propostos: %d · Sem e-mail: %d · CONFLITOS: %d\n\n",
		len(inv.Singles), len(inv.FusionCandidates), len(inv.NoEmail), len(inv.Conflicts)); err != nil {
		return err
	}

	if len(inv.FusionCandidates) > 0 {
		if err := p("## Propostas de fusão (1 identidade + N memberships — execução exige aprovação humana, T-016)\n"); err != nil {
			return err
		}
		for _, g := range inv.FusionCandidates {
			if err := p("- hash %s (%d contas):\n", hashPrefix(g.EmailHashHex), len(g.Accounts)); err != nil {
				return err
			}
			for _, r := range g.Accounts {
				if err := p("    - %s/%s —%s\n", r.Owner, r.Name, factors(r)); err != nil {
					return err
				}
			}
		}
		if err := p("\n"); err != nil {
			return err
		}
	}

	if err := p("## Conflitos para revisão humana (fusão automática silenciosa é PROIBIDA)\n"); err != nil {
		return err
	}
	if len(inv.Conflicts) == 0 {
		return p("Nenhum conflito.\n")
	}
	for _, c := range inv.Conflicts {
		if err := p("- [%s] %s\n", c.Kind, c.Detail); err != nil {
			return err
		}
		for _, r := range c.Accounts {
			if err := p("    - %s/%s —%s\n", r.Owner, r.Name, factors(r)); err != nil {
				return err
			}
		}
	}
	return nil
}
