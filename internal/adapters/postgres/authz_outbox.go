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

// AuthzOutbox writes derived authorization tuples to the transactional outbox
// (migration 0031). It is built on the SAME transaction (Querier) that persists
// the domain mutation — pass the TenantTx/IdentityTx — so the tuple deltas and the
// mutation commit or roll back together (RFC-0004 §4): the publisher (T-006)
// drains them asynchronously, the only place a remote write to the PDP is allowed.
// This is how a role/grant/group change reaches the graph WITHOUT a remote call
// inside a DB transaction, and without a projection surviving a rolled-back
// mutation.
type AuthzOutbox struct {
	q Querier
}

// NewAuthzOutbox builds the outbox writer over a Querier.
func NewAuthzOutbox(q Querier) *AuthzOutbox {
	return &AuthzOutbox{q: q}
}

// Enqueue writes the tuple updates to the outbox atomically with the caller's
// transaction. Each tuple is validated first (domain.ValidateTuple): an
// unqualified or cross-tenant tuple is REFUSED and never enqueued (INV-5), and a
// refusal rolls back the caller's mutation. organization_id is derived from the
// tuple's tenant-qualified object. An empty batch is a no-op.
func (o *AuthzOutbox) Enqueue(ctx context.Context, updates []domain.TupleUpdate) error {
	for _, u := range updates {
		if !u.Op.Valid() {
			return fmt.Errorf("authz_outbox: operação inválida %q", u.Op)
		}
		if err := domain.ValidateTuple(u.Tuple); err != nil {
			return fmt.Errorf("authz_outbox: tupla rejeitada: %w", err)
		}
		org, ok := domain.RefOrg(u.Tuple.Object)
		if !ok {
			return fmt.Errorf("authz_outbox: objeto sem tenant %q", u.Tuple.Object)
		}
		orgID, err := uuid.Parse(org)
		if err != nil {
			return fmt.Errorf("authz_outbox: organização inválida no objeto %q: %w", u.Tuple.Object, err)
		}
		const q = `
			INSERT INTO authz_tuple_outbox
				(id, organization_id, op, tuple_user, tuple_relation, tuple_object)
			VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := o.q.Exec(ctx, q,
			uuid.New().String(), orgID.String(), string(u.Op),
			u.Tuple.User, u.Tuple.Relation, u.Tuple.Object); err != nil {
			return fmt.Errorf("authz_outbox: enfileiramento da tupla falhou: %w", err)
		}
	}
	return nil
}
