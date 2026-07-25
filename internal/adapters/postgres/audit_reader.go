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
	"github.com/jackc/pgx/v5"
)

// TenantAuditReader reads a tenant's audit trail for display (the console
// timeline). It is READ-ONLY over the append-only trail (INV-2) and always
// predicates on the organization (Barreira 1) — the same explicit scope the
// verifier uses. It reuses scanSealedEvent, so it stays in lockstep with the
// stored column layout.
type TenantAuditReader struct {
	db Beginner
}

// NewTenantAuditReader builds the reader over the runtime pool.
func NewTenantAuditReader(db Beginner) *TenantAuditReader {
	return &TenantAuditReader{db: db}
}

// ListRecent returns the tenant's most recent audit events, newest first, capped
// at limit. It never mutates the trail. The column list matches scanSealedEvent's
// expected order (kept identical to the verifier's read).
func (r *TenantAuditReader) ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.SealedEvent, error) {
	var out []domain.SealedEvent
	err := WithTx(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT seq, occurred_at, event_id::text, schema_version, action, outcome,
			       actor_subject, actor_membership_id::text, actor_session_id::text, actor_act,
			       target_type, target_id, target_label, reason,
			       context_ip, context_user_agent, context_acr, context_amr, context_trace_id, context_pcid,
			       prev_hash, hash
			FROM audit_event WHERE organization_id = $1 ORDER BY seq DESC LIMIT $2`,
			orgID.String(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			se, err := scanSealedEvent(rows, orgID)
			if err != nil {
				return err
			}
			out = append(out, se)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
