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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IdentityImporter imports a batch of external identities (pacote 009, T-017 /
// RFC-0007 §4) through the SAME dedup-by-email path as SCIM/LDAP/federation, so an
// import produces a MEMBERSHIP for a known e-mail, never a duplicate identity. It
// NEVER imports a password: an imported identity has no credential, so first
// access requires enrolling a strong factor (enrollment_required). Each batch
// yields a report that is auditable and reversible (revoke the created memberships).
type IdentityImporter struct {
	pool      *pgxpool.Pool
	custodian domain.KeyCustodian
	prov      *DirectoryProvisioner
}

// NewIdentityImporter builds the importer over the pool and the key custodian.
func NewIdentityImporter(pool *pgxpool.Pool, custodian domain.KeyCustodian) *IdentityImporter {
	return &IdentityImporter{pool: pool, custodian: custodian, prov: NewDirectoryProvisioner(pool, custodian)}
}

// Import imports records into orgID under batchID, returning a per-record report.
// A record whose e-mail is already known is labelled Reused (membership only, no
// new identity); a fresh e-mail is Created (credential-less identity + membership);
// a malformed or failing record is Failed with its error. One bad record never
// aborts the batch.
func (i *IdentityImporter) Import(ctx context.Context, orgID uuid.UUID, batchID string, records []domain.ImportRecord) (domain.ImportReport, error) {
	report := domain.ImportReport{BatchID: batchID}
	identities := NewIdentityStore(i.pool)

	for _, rec := range records {
		entry := domain.ImportEntry{Email: rec.Email}
		if err := rec.Validate(); err != nil {
			entry.Outcome = domain.ImportFailed
			entry.Error = err.Error()
			report.Entries = append(report.Entries, entry)
			continue
		}

		// Classify created vs reused by whether the e-mail already resolves.
		outcome := domain.ImportCreated
		if _, err := identities.FindByEmail(ctx, i.custodian, rec.Email); err == nil {
			outcome = domain.ImportReused
		} else if !errors.Is(err, ErrIdentityNotFound) {
			entry.Outcome = domain.ImportFailed
			entry.Error = err.Error()
			report.Entries = append(report.Entries, entry)
			continue
		}

		id, err := i.prov.ProvisionUser(ctx, orgID, rec.ToSyncRecord())
		if err != nil {
			entry.Outcome = domain.ImportFailed
			entry.Error = err.Error()
			report.Entries = append(report.Entries, entry)
			continue
		}
		entry.IdentityID = id
		entry.Outcome = outcome
		report.Entries = append(report.Entries, entry)
	}
	return report, nil
}
