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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// FactorRemover removes an identity's authentication factor and records a
// factor.remove audit event ATOMICALLY in the same transaction (T-016). Removing
// a strong factor is an L3 operation (spec "Tentativa de reset silencioso": the
// operation is L3, the affected identity is notified, the event is audited) — the
// L3 gate is enforced at the API boundary by the assurance middleware; the audit
// and the affected-identity notification are recorded here. The audit shares the
// removal's transaction, so an unrecorded removal never commits (I-5.4).
type FactorRemover struct {
	repo  *TenantRepository
	audit AuditEmitter
}

// NewFactorRemover builds the remover over the admin's tenant repository (the
// organization whose audit chain records the action) and the audit emitter. A
// nil emitter leaves the operation uninstrumented (dev/tests only).
func NewFactorRemover(repo *TenantRepository, audit AuditEmitter) *FactorRemover {
	return &FactorRemover{repo: repo, audit: audit}
}

// RemoveStrongFactor deletes credential credID of targetIdentity and emits a
// factor.remove event whose actor is the acting principal (from the context) and
// whose target is the affected identity's OPAQUE subject (non-personal). The
// removal and the audit share one transaction: a missing principal or an audit
// failure rolls back the removal (fail-closed). ErrCredentialNotFound is returned
// (and nothing audited) when the factor did not exist.
func (r *FactorRemover) RemoveStrongFactor(ctx context.Context, targetIdentity, credID uuid.UUID, targetSubject string) error {
	return r.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		if err := NewCredentialStore(ttx.Tx()).Remove(ctx, credID, targetIdentity); err != nil {
			return err
		}
		return emitAudit(ctx, ttx.Tx(), r.audit, r.repo.Scope().OrganizationID(),
			domain.ActionFactorRemove,
			domain.AuditTarget{Type: "identity", ID: targetSubject, Label: "fator forte"},
			"remoção de fator forte")
	})
}
