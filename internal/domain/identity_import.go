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

import "errors"

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
)

// ImportEntry is the per-record result.
type ImportEntry struct {
	Email      string
	IdentityID string
	Outcome    ImportOutcome
	Error      string
}

// ImportReport is the auditable, reversible result of one import batch. Reversal is
// revoking the memberships created under BatchID (RFC-0007 §4/§7).
type ImportReport struct {
	BatchID string
	Entries []ImportEntry
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
