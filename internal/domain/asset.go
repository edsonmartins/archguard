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

package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Asset is a privileged target of a tenant — a host, a service, a target account,
// whatever granularity the deployment settles on. That granularity is an RFC-0004
// §9 open question (M4), so Kind is a FREE label, not a closed enum: this type
// stays agnostic rather than inventing the answer. Likewise ExternalRef is the
// opaque identifier of the same resource in the brokering component
// (Warpgate/NetBird); it is empty when the asset is registered directly. The
// canonical identity is ArchGuard's own ID (RFC-0004 §9 tentative answer).
//
// An asset belongs to at most one parent AssetGroup — the tree edge inheritance
// flows through — and may name an owner membership.
type Asset struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	// Kind is a free-form granularity label ("host", "service", "account", …). Not
	// a closed enum on purpose (RFC-0004 §9.2 is open).
	Kind string
	Name string
	// ExternalRef is the opaque id of this resource in the brokering component; ""
	// for a directly-registered asset. Never a secret (INV-7).
	ExternalRef string
	// ParentGroupID is the asset_group this asset belongs to (same tenant). nil =
	// ungrouped.
	ParentGroupID *uuid.UUID
	// OwnerMembershipID is the membership that owns the asset (same tenant), if any.
	OwnerMembershipID *uuid.UUID
}

// ErrInvalidAsset is returned when required asset data is missing.
var ErrInvalidAsset = errors.New("asset: dados obrigatórios ausentes")

// NewAsset builds a validated asset (UUIDv7 id). organization, kind and name are
// required; externalRef, parent group and owner are optional. Parent/owner are
// same-tenant references — their tenant is this asset's organization.
func NewAsset(organizationID uuid.UUID, kind, name, externalRef string, parentGroupID, ownerMembershipID *uuid.UUID) (Asset, error) {
	if organizationID == uuid.Nil {
		return Asset{}, fmt.Errorf("%w: organização", ErrInvalidAsset)
	}
	if kind == "" {
		return Asset{}, fmt.Errorf("%w: tipo", ErrInvalidAsset)
	}
	if name == "" {
		return Asset{}, fmt.Errorf("%w: nome", ErrInvalidAsset)
	}
	if parentGroupID != nil && *parentGroupID == uuid.Nil {
		return Asset{}, fmt.Errorf("%w: parent nulo explícito", ErrInvalidAsset)
	}
	if ownerMembershipID != nil && *ownerMembershipID == uuid.Nil {
		return Asset{}, fmt.Errorf("%w: owner nulo explícito", ErrInvalidAsset)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Asset{}, fmt.Errorf("asset: geração de UUIDv7 falhou: %w", err)
	}
	return Asset{
		ID:                id,
		OrganizationID:    organizationID,
		Kind:              kind,
		Name:              name,
		ExternalRef:       externalRef,
		ParentGroupID:     parentGroupID,
		OwnerMembershipID: ownerMembershipID,
	}, nil
}

// Ref is the tenant-qualified graph ref of this asset (T-003).
func (a Asset) Ref() string { return Qualify(a.OrganizationID, TypeAsset, a.ID.String()) }

// Tuples projects the asset's structural relations to authorization tuples — the
// `parent` edge (into its group) and the `owner` edge (to its owner membership).
// Operator/auditor assignments come from access policies and role assignments and
// are projected elsewhere (T-007). Every tuple returned is intra-tenant and passes
// ValidateTuple by construction.
func (a Asset) Tuples() []RelationTuple {
	var out []RelationTuple
	if a.ParentGroupID != nil {
		parent := Qualify(a.OrganizationID, TypeAssetGroup, a.ParentGroupID.String())
		out = append(out, RelationTuple{User: parent, Relation: RelParent, Object: a.Ref()})
	}
	if a.OwnerMembershipID != nil {
		owner := Qualify(a.OrganizationID, TypeMembership, a.OwnerMembershipID.String())
		out = append(out, RelationTuple{User: owner, Relation: RelOwner, Object: a.Ref()})
	}
	return out
}
