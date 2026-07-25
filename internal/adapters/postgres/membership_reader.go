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

// MembershipReader lists an identity's memberships across every tenant — a
// cross-tenant read, so it goes through the GlobalRepository (authorized and
// audited, INV-6/I-5.4), the same path SessionBridge uses at login. The console's
// tenant selector uses it to show which tenants the caller can act in.
type MembershipReader struct {
	global *GlobalRepository
}

// NewMembershipReader builds the reader over the global repository.
func NewMembershipReader(global *GlobalRepository) *MembershipReader {
	return &MembershipReader{global: global}
}

// ListByIdentity returns every membership of the identity, across tenants. The
// access is declared as the caller reading its own tenant list; a global-access
// denial (e.g. a conformant profile without the durable authorizer wired) or a
// query failure is propagated — fail-closed, never a partial list served as whole.
func (r *MembershipReader) ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]domain.Membership, error) {
	var out []domain.Membership
	err := r.global.WithGlobalTx(ctx, domain.GlobalAccess{
		Principal: identityID.String(),
		Reason:    "console: listar os tenants do próprio chamador",
	}, func(tx pgx.Tx) error {
		ms, err := NewMembershipStore(tx).ListByIdentity(ctx, identityID)
		if err != nil {
			return err
		}
		out = ms
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
