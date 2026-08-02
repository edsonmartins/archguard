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

// AccessGroup names an access group of the tenant (pacote 007 M4, D1 catálogo) — the
// human-readable catalog behind the otherwise-opaque group id used by group_membership
// and by group-subject access assignments (`group:<id>#member`). It is metadata only: a
// group produces NO authorization tuple by itself; membership `member` tuples and the
// group's operator/auditor assignments are what carry access.
type AccessGroup struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	DisplayName    string
}

// ErrInvalidAccessGroup is returned when the group is malformed.
var ErrInvalidAccessGroup = errors.New("access_group: dados obrigatórios ausentes")

// NewAccessGroup builds a validated access group (UUIDv7 id). name is required and
// unique per tenant; displayName is optional (defaults to name).
func NewAccessGroup(orgID uuid.UUID, name, displayName string) (AccessGroup, error) {
	if orgID == uuid.Nil || name == "" {
		return AccessGroup{}, fmt.Errorf("%w: organização e nome", ErrInvalidAccessGroup)
	}
	if displayName == "" {
		displayName = name
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AccessGroup{}, fmt.Errorf("access_group: geração de UUIDv7 falhou: %w", err)
	}
	return AccessGroup{ID: id, OrganizationID: orgID, Name: name, DisplayName: displayName}, nil
}

// Ref is the tenant-qualified graph ref of the group (the object of `member` tuples).
func (g AccessGroup) Ref() string { return Qualify(g.OrganizationID, TypeGroup, g.ID.String()) }
