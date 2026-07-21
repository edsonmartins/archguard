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
)

// SessionOutbox writes session audit events to the transactional outbox
// (migration 0016). It is built on the SAME transaction that persists the
// business operation, so the event and the operation commit or roll back
// together (RFC-0004 §4 transactional outbox): the durable, tamper-evident
// trail (pacote 003) drains this table asynchronously — outside any business
// transaction, where a remote call is allowed. This is what lets the switch
// satisfy I-5.4 (audit-or-deny) without ever making a remote call inside a DB
// transaction.
type SessionOutbox struct {
	q Querier
}

// NewSessionOutbox builds the outbox writer over a Querier — pass the IdentityTx
// (or TenantTx) transaction so the enqueue is atomic with the operation.
func NewSessionOutbox(q Querier) *SessionOutbox {
	return &SessionOutbox{q: q}
}

// EnqueueTenantSwitch writes one tenant-switch event to the outbox. The event
// is validated first (a malformed event is never enqueued). A failed insert
// surfaces as an error, so the caller's transaction rolls back and the switch
// is denied — an unrecorded switch does not happen.
func (o *SessionOutbox) EnqueueTenantSwitch(ctx context.Context, event domain.TenantSwitchEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	const q = `
		INSERT INTO session_event_outbox
			(id, event_type, session_id, identity_id,
			 from_membership_id, from_organization_id, to_membership_id, to_organization_id,
			 proven_aal, token_generation)
		VALUES ($1, 'tenant_switch', $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := o.q.Exec(ctx, q,
		uuid.New().String(), event.SessionID.String(), event.IdentityID.String(),
		event.FromMembershipID.String(), event.FromOrganizationID.String(),
		event.ToMembershipID.String(), event.ToOrganizationID.String(),
		string(event.ProvenAAL), event.TokenGeneration)
	if err != nil {
		return fmt.Errorf("postgres: gravação do evento de troca no outbox falhou: %w", err)
	}
	return nil
}
