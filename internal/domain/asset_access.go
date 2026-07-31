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

// AssetAccessAssignment is the source-of-truth record that a subject (a membership)
// holds a relation (operator/auditor) over an object (an asset or an asset_group) —
// the GRANULAR access authorization (pacote 007 M4, T-029). The projection derives
// the corresponding graph tuple; the PDP then computes can_open_session (operator or
// owner) and, via the object's `parent` chain, the INHERITED access of an asset_group
// to its child assets.
type AssetAccessAssignment struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	SubjectID      uuid.UUID  // a membership (same tenant)
	Relation       string     // operator | auditor
	ObjectType     ObjectType // asset | asset_group
	ObjectID       uuid.UUID  // same tenant
}

// ErrInvalidAssetAccess is returned when the assignment is malformed.
var ErrInvalidAssetAccess = errors.New("asset_access: dados obrigatórios ausentes ou inválidos")

// NewAssetAccessAssignment builds a validated assignment (UUIDv7 id). The relation
// must be assignable (operator/auditor — can_open_* are derived, never assigned) and
// the object must be an asset or an asset_group.
func NewAssetAccessAssignment(orgID, subjectID uuid.UUID, relation string, objectType ObjectType, objectID uuid.UUID) (AssetAccessAssignment, error) {
	if orgID == uuid.Nil || subjectID == uuid.Nil || objectID == uuid.Nil {
		return AssetAccessAssignment{}, fmt.Errorf("%w: organização, sujeito e objeto", ErrInvalidAssetAccess)
	}
	if relation != RelOperator && relation != RelAuditor {
		return AssetAccessAssignment{}, fmt.Errorf("%w: relação %q não é atribuível (use operator/auditor)", ErrInvalidAssetAccess, relation)
	}
	if objectType != TypeAsset && objectType != TypeAssetGroup {
		return AssetAccessAssignment{}, fmt.Errorf("%w: objeto %q deve ser asset ou asset_group", ErrInvalidAssetAccess, objectType)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AssetAccessAssignment{}, fmt.Errorf("asset_access: geração de UUIDv7 falhou: %w", err)
	}
	return AssetAccessAssignment{
		ID:             id,
		OrganizationID: orgID,
		SubjectID:      subjectID,
		Relation:       relation,
		ObjectType:     objectType,
		ObjectID:       objectID,
	}, nil
}

// SubjectRef is the tenant-qualified graph ref of the subject membership.
func (a AssetAccessAssignment) SubjectRef() string {
	return Qualify(a.OrganizationID, TypeMembership, a.SubjectID.String())
}

// ObjectRef is the tenant-qualified graph ref of the target asset/asset_group.
func (a AssetAccessAssignment) ObjectRef() string {
	return Qualify(a.OrganizationID, a.ObjectType, a.ObjectID.String())
}

// Tuple projects the assignment to its authorization TupleUpdate (present → write,
// absent → delete), reusing the shared ProjectRoleAssignment derivation.
func (a AssetAccessAssignment) Tuple(present bool) (TupleUpdate, error) {
	return ProjectRoleAssignment(a.ObjectRef(), a.Relation, a.SubjectRef(), present)
}
