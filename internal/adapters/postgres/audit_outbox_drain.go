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
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SwitchOutboxDrainer drains the session_event_outbox (the transactional outbox
// the tenant switch writes, pacote 002) into the immutable audit chain (T-009,
// closing that loop). A tenant switch is a non-privileged (L2) event, so it
// belongs on the async path: the switch commits fast with only the outbox row,
// and this drainer later seals a `tenant.switch` audit event into the chain,
// marking the outbox row published in the SAME transaction (at-least-once, never
// lost, never double-chained).
type SwitchOutboxDrainer struct{}

// NewSwitchOutboxDrainer builds the drainer.
func NewSwitchOutboxDrainer() *SwitchOutboxDrainer { return &SwitchOutboxDrainer{} }

type switchOutboxRow struct {
	id         string
	sessionID  string
	identityID string
	fromMemID  string
	fromOrgID  string
	toMemID    string
	toOrgID    string
	provenAAL  string
	tokenGen   int
	occurredAt time.Time
}

// Drain seals up to batch unpublished switch events into the chain, oldest
// first, and returns how many were sealed. Each event is sealed and its outbox
// row marked published in one transaction.
func (d *SwitchOutboxDrainer) Drain(ctx context.Context, db Beginner, batch int) (int, error) {
	var rowsBatch []switchOutboxRow
	if err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, session_id::text, identity_id::text,
			       from_membership_id::text, from_organization_id::text,
			       to_membership_id::text, to_organization_id::text,
			       proven_aal, token_generation, occurred_at
			FROM session_event_outbox
			WHERE event_type = 'tenant_switch' AND published_at IS NULL
			ORDER BY occurred_at, id
			LIMIT $1`, batch)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r switchOutboxRow
			if err := rows.Scan(&r.id, &r.sessionID, &r.identityID, &r.fromMemID, &r.fromOrgID,
				&r.toMemID, &r.toOrgID, &r.provenAAL, &r.tokenGen, &r.occurredAt); err != nil {
				return err
			}
			rowsBatch = append(rowsBatch, r)
		}
		return rows.Err()
	}); err != nil {
		return 0, fmt.Errorf("audit: leitura do outbox de sessão falhou: %w", err)
	}

	drained := 0
	for _, r := range rowsBatch {
		event, err := d.buildEvent(ctx, db, r)
		if err != nil {
			return drained, err
		}
		if err := WithTx(ctx, db, func(tx pgx.Tx) error {
			if _, err := sealEventInTx(ctx, tx, event); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx,
				`UPDATE session_event_outbox SET published_at = now()
				 WHERE id = $1 AND published_at IS NULL`, r.id)
			if err != nil {
				return fmt.Errorf("audit: marcação de publicado falhou: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("audit: evento de outbox %s já publicado concorrentemente", r.id)
			}
			return nil
		}); err != nil {
			return drained, err
		}
		drained++
	}
	return drained, nil
}

// buildEvent maps one outbox row to a `tenant.switch` audit event, resolving the
// actor's opaque subject from the identity id (the trail carries the pseudonym,
// never the internal id). occurred_at is the switch time captured in the outbox.
func (d *SwitchOutboxDrainer) buildEvent(ctx context.Context, db Beginner, r switchOutboxRow) (domain.AuditEvent, error) {
	subject, err := lookupSubject(ctx, db, r.identityID)
	if err != nil {
		return domain.AuditEvent{}, err
	}
	toOrg, err := uuid.Parse(r.toOrgID)
	if err != nil {
		return domain.AuditEvent{}, fmt.Errorf("audit: to_organization_id inválido %q: %w", r.toOrgID, err)
	}
	toMem, err := uuid.Parse(r.toMemID)
	if err != nil {
		return domain.AuditEvent{}, fmt.Errorf("audit: to_membership_id inválido %q: %w", r.toMemID, err)
	}
	sid, err := uuid.Parse(r.sessionID)
	if err != nil {
		return domain.AuditEvent{}, fmt.Errorf("audit: session_id inválido %q: %w", r.sessionID, err)
	}
	in := domain.AuditEventInput{
		OrganizationID: toOrg,
		Action:         domain.ActionTenantSwitch,
		Actor: domain.AuditActor{
			IdentitySubject: subject,
			MembershipID:    &toMem,
			SessionID:       &sid,
		},
		Outcome: domain.Allowed,
		Target:  domain.AuditTarget{Type: "organization", ID: r.toOrgID, Label: "tenant ativo"},
		Reason:  fmt.Sprintf("troca de tenant %s → %s (geração de token %d)", r.fromOrgID, r.toOrgID, r.tokenGen),
		Context: domain.AuditContext{AuthContextClass: aalToACR(r.provenAAL)},
	}
	event, err := domain.NewAuditEvent(in)
	if err != nil {
		return domain.AuditEvent{}, err
	}
	event.OccurredAt = r.occurredAt.UTC()
	return event, nil
}

// lookupSubject resolves an identity's opaque subject by its id.
func lookupSubject(ctx context.Context, db Beginner, identityID string) (string, error) {
	var subject string
	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT subject FROM identity WHERE id = $1`, identityID).Scan(&subject)
	})
	if err != nil {
		return "", fmt.Errorf("audit: subject da identidade %s não resolvido: %w", identityID, err)
	}
	return subject, nil
}

// aalToACR maps the proven AAL (aal1/2/3) to the acr class (L1/L2/L3) the audit
// context carries.
func aalToACR(aal string) string {
	switch aal {
	case "aal3":
		return "L3"
	case "aal2":
		return "L2"
	default:
		return "L1"
	}
}
