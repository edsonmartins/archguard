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

// AssetGroup is a node of the asset hierarchy that inheritance flows through
// ("operator from parent", RFC-0004 §3). A group may nest inside a parent group,
// forming the tree "cluster → group → asset" that eliminates the combinatorial
// explosion the ADR-0005 rejects. It is a PURE domain entity: how (or whether)
// assets are imported from the brokering components and how they persist are
// RFC-0004 §9 open questions deferred to M4 — this type does not decide them.
type AssetGroup struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	// ParentGroupID nests this group inside another (same tenant). nil = top-level.
	ParentGroupID *uuid.UUID
}

// Errors of asset-hierarchy modeling.
var (
	ErrInvalidAssetGroup = errors.New("asset_group: dados obrigatórios ausentes")
	// ErrAssetGroupCycle is returned when the parent chain among groups forms a
	// cycle — a hierarchy that is not a tree. The resolver already terminates on a
	// cycle at decision time (fail-closed), but a cycle must never be persisted.
	ErrAssetGroupCycle = errors.New("asset_group: ciclo na hierarquia de grupos")
	// ErrAssetGroupSelfParent is returned when a group is its own parent.
	ErrAssetGroupSelfParent = errors.New("asset_group: grupo não pode ser pai de si mesmo")
)

// NewAssetGroup builds a validated group. Timestamps/ids are generated here
// (UUIDv7). A nil parent means top-level; a parent equal to the new id is
// impossible at creation (fresh id), but a later re-parent is guarded by
// ValidateAssetGroupHierarchy.
func NewAssetGroup(organizationID uuid.UUID, name string, parentGroupID *uuid.UUID) (AssetGroup, error) {
	if organizationID == uuid.Nil {
		return AssetGroup{}, fmt.Errorf("%w: organização", ErrInvalidAssetGroup)
	}
	if name == "" {
		return AssetGroup{}, fmt.Errorf("%w: nome", ErrInvalidAssetGroup)
	}
	if parentGroupID != nil && *parentGroupID == uuid.Nil {
		return AssetGroup{}, fmt.Errorf("%w: parent nulo explícito", ErrInvalidAssetGroup)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AssetGroup{}, fmt.Errorf("asset_group: geração de UUIDv7 falhou: %w", err)
	}
	if parentGroupID != nil && *parentGroupID == id {
		return AssetGroup{}, ErrAssetGroupSelfParent
	}
	return AssetGroup{ID: id, OrganizationID: organizationID, Name: name, ParentGroupID: parentGroupID}, nil
}

// Ref is the tenant-qualified graph ref of this group (T-003).
func (g AssetGroup) Ref() string { return Qualify(g.OrganizationID, TypeAssetGroup, g.ID.String()) }

// Tuples projects the group's structural relations to authorization tuples — the
// `parent` edge when nested. Group MEMBERSHIP of operators/auditors is projected
// elsewhere (from role assignments, T-007); this covers only the hierarchy edge.
func (g AssetGroup) Tuples() []RelationTuple {
	if g.ParentGroupID == nil {
		return nil
	}
	parent := Qualify(g.OrganizationID, TypeAssetGroup, g.ParentGroupID.String())
	return []RelationTuple{{User: parent, Relation: RelParent, Object: g.Ref()}}
}

// ValidateAssetGroupHierarchy checks that the parent relation among the given
// groups is a proper tree within one tenant: no cycle, no self-parent, no
// dangling or cross-tenant parent. groups is keyed by id. It is the integrity gate
// a persistence/import step (M4) runs before accepting a re-parenting.
func ValidateAssetGroupHierarchy(groups map[uuid.UUID]AssetGroup) error {
	for id, g := range groups {
		if g.ParentGroupID == nil {
			continue
		}
		if *g.ParentGroupID == id {
			return ErrAssetGroupSelfParent
		}
		// Walk up the parent chain, bounded by the number of groups; revisiting a
		// node (or exceeding the bound) means a cycle.
		seen := map[uuid.UUID]bool{id: true}
		cur := *g.ParentGroupID
		for steps := 0; steps <= len(groups); steps++ {
			parent, ok := groups[cur]
			if !ok {
				return fmt.Errorf("%w: parent inexistente %s", ErrInvalidAssetGroup, cur)
			}
			if parent.OrganizationID != g.OrganizationID {
				return ErrCrossTenantRelation
			}
			if seen[cur] {
				return ErrAssetGroupCycle
			}
			seen[cur] = true
			if parent.ParentGroupID == nil {
				break
			}
			cur = *parent.ParentGroupID
		}
	}
	return nil
}
