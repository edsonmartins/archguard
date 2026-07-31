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

// AssetStore persists the tenant's asset catalog (pacote 007 M4, T-026). It is built
// on a TenantTx, so every operation is confined to one tenant (RLS + scope). On each
// mutation it ENQUEUES the derived authorization tuples to the AuthzOutbox in the SAME
// transaction (RFC-0004 §4: the projection intent is atomic with the change; the
// publisher drains it to authz_tuple later, never a remote call inside the tx).
type AssetStore struct {
	ttx *TenantTx
}

// NewAssetStore builds the store on an open tenant transaction. No constructor
// without a tenant.
func NewAssetStore(ttx *TenantTx) *AssetStore {
	return &AssetStore{ttx: ttx}
}

// enqueue writes the derived tuple updates to the outbox in this transaction.
func (s *AssetStore) enqueue(ctx context.Context, updates []domain.TupleUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	return NewAuthzOutbox(s.ttx.Tx()).Enqueue(ctx, updates)
}

// CreateGroup inserts an asset group and enqueues its structural tuples (the `parent`
// edge, when nested). Refuses a group whose tenant differs from the store's.
func (s *AssetStore) CreateGroup(ctx context.Context, g domain.AssetGroup) error {
	scope := s.ttx.scope.OrganizationID()
	if g.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, g.OrganizationID, scope)
	}
	const q = `INSERT INTO asset_group (id, organization_id, name, parent_group_id)
	           VALUES ($1, $2, $3, $4)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		g.ID.String(), g.OrganizationID.String(), g.Name, uuidPtrArg(g.ParentGroupID)); err != nil {
		return fmt.Errorf("postgres: criação de asset_group falhou: %w", err)
	}
	// projeta a existência das arestas do grupo (o `parent`, se houver) — TupleWrite.
	updates := make([]domain.TupleUpdate, 0, len(g.Tuples()))
	for _, t := range g.Tuples() {
		updates = append(updates, domain.TupleUpdate{Op: domain.TupleWrite, Tuple: t})
	}
	return s.enqueue(ctx, updates)
}

// Create inserts an asset and enqueues its structural tuples (parent/owner edges).
// Refuses an asset whose tenant differs from the store's.
func (s *AssetStore) Create(ctx context.Context, a domain.Asset) error {
	scope := s.ttx.scope.OrganizationID()
	if a.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, a.OrganizationID, scope)
	}
	const q = `INSERT INTO asset (id, organization_id, kind, name, external_ref, parent_group_id, owner_membership_id)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		a.ID.String(), a.OrganizationID.String(), a.Kind, a.Name, a.ExternalRef,
		uuidPtrArg(a.ParentGroupID), uuidPtrArg(a.OwnerMembershipID)); err != nil {
		return fmt.Errorf("postgres: criação de asset falhou: %w", err)
	}
	return s.enqueue(ctx, domain.ProjectAsset(a, true))
}

// List returns the tenant's assets, newest first.
func (s *AssetStore) List(ctx context.Context) ([]domain.Asset, error) {
	const q = `SELECT id, organization_id, kind, name, external_ref, parent_group_id, owner_membership_id
	           FROM asset ORDER BY created_at DESC`
	rows, err := s.ttx.tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de assets falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.Asset
	for rows.Next() {
		var a domain.Asset
		var idStr, orgStr string
		var parent, owner *string
		if err := rows.Scan(&idStr, &orgStr, &a.Kind, &a.Name, &a.ExternalRef, &parent, &owner); err != nil {
			return nil, fmt.Errorf("postgres: scan de asset falhou: %w", err)
		}
		a.ID = uuid.MustParse(idStr)
		a.OrganizationID = uuid.MustParse(orgStr)
		a.ParentGroupID = parseUUIDPtr(parent)
		a.OwnerMembershipID = parseUUIDPtr(owner)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListGroups returns the tenant's asset groups, newest first.
func (s *AssetStore) ListGroups(ctx context.Context) ([]domain.AssetGroup, error) {
	const q = `SELECT id, organization_id, name, parent_group_id FROM asset_group ORDER BY created_at DESC`
	rows, err := s.ttx.tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de asset_groups falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.AssetGroup
	for rows.Next() {
		var g domain.AssetGroup
		var idStr, orgStr string
		var parent *string
		if err := rows.Scan(&idStr, &orgStr, &g.Name, &parent); err != nil {
			return nil, fmt.Errorf("postgres: scan de asset_group falhou: %w", err)
		}
		g.ID = uuid.MustParse(idStr)
		g.OrganizationID = uuid.MustParse(orgStr)
		g.ParentGroupID = parseUUIDPtr(parent)
		out = append(out, g)
	}
	return out, rows.Err()
}

// uuidPtrArg renders a *uuid.UUID as a nullable text arg (nil → NULL).
func uuidPtrArg(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}

// parseUUIDPtr parses a nullable uuid text column back to *uuid.UUID.
func parseUUIDPtr(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id := uuid.MustParse(*s)
	return &id
}
