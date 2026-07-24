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
	"github.com/jackc/pgx/v5"
)

// AuthzBootstrap reconstructs the PDP projection store (authz_tuple) from the
// source of truth (RFC-0004 §4: "reconstruir o store do zero a partir do banco").
// This is the disaster-recovery and PDP-swap capability: given the tenant's
// authoritative derived tuple set, it REPLACES the tenant's store contents
// atomically, so the result is deterministic and equal to the source projection —
// re-running it, or reconciling against the same expected set afterwards, finds no
// divergence. A full-store bootstrap iterates tenants.
type AuthzBootstrap struct{}

// NewAuthzBootstrap builds the bootstrapper.
func NewAuthzBootstrap() *AuthzBootstrap { return &AuthzBootstrap{} }

// RebuildTenant replaces org's projection with exactly `expected`, in ONE
// transaction: the old rows are deleted and the expected rows inserted, so a
// failure leaves the store unchanged (atomic rebuild). Every expected tuple must
// belong to org (checked via the tenant-qualified object) — a stray tuple aborts
// the rebuild rather than planting a cross-tenant row (INV-5). Returns how many
// tuples the store holds for org afterwards.
func (b *AuthzBootstrap) RebuildTenant(ctx context.Context, db Beginner, orgID uuid.UUID, expected []domain.RelationTuple) (int, error) {
	// De-duplicate the expected set on tuple identity before the rebuild.
	deduped := make(map[string]domain.RelationTuple, len(expected))
	for _, tup := range expected {
		if !tup.Valid() {
			return 0, fmt.Errorf("authz_bootstrap: tupla esperada incompleta: %+v", tup)
		}
		org, ok := domain.RefOrg(tup.Object)
		if !ok || org != orgID.String() {
			return 0, fmt.Errorf("authz_bootstrap: tupla esperada fora do tenant %s: %+v", orgID, tup)
		}
		deduped[tup.User+"\x00"+tup.Relation+"\x00"+tup.Object] = tup
	}

	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM authz_tuple WHERE organization_id = $1`, orgID.String()); err != nil {
			return err
		}
		for _, tup := range deduped {
			if _, err := tx.Exec(ctx, `
				INSERT INTO authz_tuple (organization_id, tuple_user, tuple_relation, tuple_object)
				VALUES ($1, $2, $3, $4)`,
				orgID.String(), tup.User, tup.Relation, tup.Object); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("authz_bootstrap: reconstrução do tenant %s falhou: %w", orgID, err)
	}
	return len(deduped), nil
}
