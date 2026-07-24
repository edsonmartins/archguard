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

package domain

import (
	"errors"
	"fmt"
)

// Identity import (pacote 009, T-017 / RFC-0007 §4). A batch of identities from an
// external export (the PoC, an AD dump, an operator spreadsheet) is imported into
// ArchGuard. Two invariants are structural here:
//
//   - NO PASSWORD IS EVER IMPORTED: ImportRecord has no password field, so a
//     source credential cannot even be expressed. An imported identity has no
//     credential and therefore MUST enroll a strong factor on first access
//     (enrollment_required) — the spec's "nenhuma senha da origem é aceita".
//   - The person is deduplicated by email_hash: an import produces a MEMBERSHIP,
//     never a duplicate identity, when the person already exists (RFC-0002 §6).

// ImportRecord is one identity to import. It carries only non-secret attributes;
// there is deliberately NO password/credential field.
type ImportRecord struct {
	Email       string
	DisplayName string
	ExternalID  string
}

// ErrImportEmailRequired is returned when an import record has no e-mail (the dedup
// key and the only mandatory field).
var ErrImportEmailRequired = errors.New("import: e-mail obrigatório")

// Validate refuses a record with no e-mail.
func (r ImportRecord) Validate() error {
	if r.Email == "" {
		return ErrImportEmailRequired
	}
	return nil
}

// ToSyncRecord maps the import record to the neutral provisioning record so it
// rides the same dedup path as SCIM/LDAP/federation. Active (an imported identity
// is active but must enroll to authenticate).
func (r ImportRecord) ToSyncRecord() DirectorySyncRecord {
	external := r.ExternalID
	if external == "" {
		external = r.Email
	}
	return DirectorySyncRecord{
		ExternalID: external,
		Email:      r.Email,
		Attributes: map[string]string{"email": r.Email, "name": r.DisplayName},
		Active:     true,
	}
}

// ImportOutcome is what happened to one record.
type ImportOutcome string

const (
	// ImportCreated: a new identity (credential-less, enrollment required) + membership.
	ImportCreated ImportOutcome = "created"
	// ImportReused: the e-mail was known — only a membership was ensured (dedup).
	ImportReused ImportOutcome = "reused"
	// ImportFailed: the record could not be imported (see Error).
	ImportFailed ImportOutcome = "failed"
	// ImportConflicted: a dedup conflict blocks automatic handling — the record is
	// NOT imported and awaits human review (RFC-0002 §6, no silent auto-merge).
	ImportConflicted ImportOutcome = "conflicted"
)

// ImportConflict is a dedup situation in an import batch that must NOT be resolved
// automatically (RFC-0002 §6: "fusão automática silenciosa é proibida"). It is
// reported for human review; the assisted merge is identfusion (approval required).
type ImportConflict struct {
	Kind   ImportConflictKind
	Email  string
	Detail string
}

// ImportConflictKind classifies a batch dedup conflict.
type ImportConflictKind string

const (
	// ImportConflictIntraBatchDuplicate: the same e-mail appears more than once in
	// the batch — which record is authoritative is a human decision, never silent.
	ImportConflictIntraBatchDuplicate ImportConflictKind = "intra_batch_duplicate"
)

// DetectImportConflicts scans a batch for dedup conflicts that block automatic
// import. It flags e-mails that appear more than once (after normalization) in the
// batch: importing them silently would let one record clobber another. Conflicted
// e-mails are returned so the importer skips them and an operator resolves them.
// The comparison uses NormalizeEmail, so "A@x" and "a@x " are the same person.
func DetectImportConflicts(records []ImportRecord) []ImportConflict {
	counts := map[string]int{}
	for _, r := range records {
		if r.Email == "" {
			continue // an empty e-mail is a validation failure, not a dedup conflict
		}
		counts[NormalizeEmail(r.Email)]++
	}
	var conflicts []ImportConflict
	seen := map[string]bool{}
	for _, r := range records {
		norm := NormalizeEmail(r.Email)
		if r.Email == "" || counts[norm] < 2 || seen[norm] {
			continue
		}
		seen[norm] = true
		conflicts = append(conflicts, ImportConflict{
			Kind:   ImportConflictIntraBatchDuplicate,
			Email:  r.Email,
			Detail: fmt.Sprintf("e-mail aparece %d vezes no lote — revisão humana antes de importar", counts[norm]),
		})
	}
	return conflicts
}

// ConflictedEmails returns the set of normalized e-mails under conflict, for the
// importer to skip.
func ConflictedEmails(conflicts []ImportConflict) map[string]bool {
	out := make(map[string]bool, len(conflicts))
	for _, c := range conflicts {
		out[NormalizeEmail(c.Email)] = true
	}
	return out
}

// ImportEntry is the per-record result.
type ImportEntry struct {
	Email      string
	IdentityID string
	Outcome    ImportOutcome
	Error      string
}

// ImportReport is the auditable, reversible result of one import batch. Reversal is
// revoking the memberships created under BatchID (RFC-0007 §4/§7). Conflicts are
// the dedup situations that were NOT auto-resolved and await human review.
type ImportReport struct {
	BatchID   string
	Entries   []ImportEntry
	Conflicts []ImportConflict
}

// Count returns how many entries had the given outcome.
func (r ImportReport) Count(o ImportOutcome) int {
	n := 0
	for _, e := range r.Entries {
		if e.Outcome == o {
			n++
		}
	}
	return n
}
