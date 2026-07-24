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
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubjectExporter serves a subject-access request (pacote 010, T-021 / spec
// "Atendimento a direitos do titular com isolamento"): it exports a titular's data
// for ONE organization, decrypting the global identity fields via the per-subject
// cipher and reading the membership of THAT organization only — never another
// tenant's data (the scoping is enforced by the tenant-bound membership store).
type SubjectExporter struct {
	pool   *pgxpool.Pool
	cipher domain.SubjectCipher
}

// NewSubjectExporter builds the exporter over the pool and the per-subject cipher.
func NewSubjectExporter(pool *pgxpool.Pool, cipher domain.SubjectCipher) *SubjectExporter {
	return &SubjectExporter{pool: pool, cipher: cipher}
}

// Export builds the export document for the subject in orgID. The global identity
// is resolved by subject (pseudonym) and its e-mail/display name are decrypted with
// the subject's key; the membership is read within orgID's tenant scope, so a
// membership in any OTHER organization is unreachable (Barreira 1). A destroyed
// (erased) subject key yields empty personal fields, not an error — an erased
// titular has no personal data to export.
func (e *SubjectExporter) Export(ctx context.Context, subject string, orgID uuid.UUID) (domain.SubjectExportDocument, error) {
	idn, err := NewIdentityStore(e.pool).FindBySubject(ctx, subject)
	if err != nil {
		return domain.SubjectExportDocument{}, fmt.Errorf("subject_export: identidade não resolvida: %w", err)
	}

	exportedID := domain.ExportedIdentity{
		Subject:     idn.Subject,
		Email:       e.decrypt(ctx, subject, idn.PrimaryEmailEnc),
		DisplayName: e.decrypt(ctx, subject, idn.DisplayNameEnc),
		Type:        string(idn.Type),
		Status:      string(idn.Status),
	}

	scope, err := domain.NewTenantScope(orgID)
	if err != nil {
		return domain.SubjectExportDocument{}, err
	}
	var mem domain.ExportedMembership
	err = NewTenantRepository(e.pool, scope).WithTenantTx(ctx, func(ttx *TenantTx) error {
		m, ferr := NewTenantMembershipStore(ttx).FindByIdentity(ctx, idn.ID)
		if ferr != nil {
			return ferr
		}
		mem = domain.ExportedMembership{OrganizationID: m.OrganizationID.String(), Status: string(m.Status)}
		return nil
	})
	if err != nil {
		return domain.SubjectExportDocument{}, fmt.Errorf("subject_export: membership da organização não resolvido: %w", err)
	}

	return domain.BuildSubjectExport(exportedID, mem), nil
}

// decrypt returns the plaintext of an encrypted personal field, or "" when there
// is nothing (nil ciphertext) or the subject key was destroyed (erased titular).
func (e *SubjectExporter) decrypt(ctx context.Context, subject string, enc []byte) string {
	if len(enc) == 0 {
		return ""
	}
	pt, err := e.cipher.DecryptForSubject(subject, enc)
	if err != nil {
		return "" // erased (ErrSubjectKeyDestroyed) or unreadable — nothing to export
	}
	return string(pt)
}
