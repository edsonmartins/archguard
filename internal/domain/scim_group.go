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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SCIM 2.0 inbound — Group resource (pacote 009, T-008 / RFC 7643). A provisioned
// group tells ArchGuard which users belong to a directory group. Group membership
// is DIRECTORY-authoritative, but a privileged role is NEVER auto-derived from a
// group without an approved mapping (design 009): this file only carries the group
// and its members, neutrally.

// SCIMGroupSchema is the core Group schema URN.
const SCIMGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"

// SCIMMember is a member reference of a SCIM group (the "value" is the member's
// resource id at the IdP).
type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

// SCIMGroup is the SCIM 2.0 Group resource (the subset ArchGuard consumes).
type SCIMGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id,omitempty"`
	ExternalID  string       `json:"externalId,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []SCIMMember `json:"members,omitempty"`
	Meta        *SCIMMeta    `json:"meta,omitempty"`
}

// GroupSyncRecord is the neutral provisioning record for a group — the group and
// its members' external ids — shared by SCIM and (future) directory group sync.
type GroupSyncRecord struct {
	ExternalID  string
	DisplayName string
	MemberIDs   []string
}

// Errors of SCIM group parsing/validation.
var (
	ErrSCIMGroupSchema      = errors.New("scim: schema de Group ausente")
	ErrSCIMGroupDisplayName = errors.New("scim: displayName obrigatório")
)

// ParseSCIMGroup decodes and validates a SCIM Group payload: the Group schema and
// a displayName are required. Members may be empty (an empty group).
func ParseSCIMGroup(body []byte) (SCIMGroup, error) {
	var g SCIMGroup
	if err := json.Unmarshal(body, &g); err != nil {
		return SCIMGroup{}, fmt.Errorf("%w: %v", ErrSCIMMalformed, err)
	}
	if !containsSchema(g.Schemas, SCIMGroupSchema) {
		return SCIMGroup{}, ErrSCIMGroupSchema
	}
	if strings.TrimSpace(g.DisplayName) == "" {
		return SCIMGroup{}, ErrSCIMGroupDisplayName
	}
	return g, nil
}

// ToGroupRecord maps the SCIM group to the neutral record. ExternalID prefers the
// IdP's externalId, falling back to displayName.
func (g SCIMGroup) ToGroupRecord() GroupSyncRecord {
	external := g.ExternalID
	if external == "" {
		external = g.DisplayName
	}
	members := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		if m.Value != "" {
			members = append(members, m.Value)
		}
	}
	return GroupSyncRecord{ExternalID: external, DisplayName: g.DisplayName, MemberIDs: members}
}

// ResponseGroup fills the resource for a SCIM response (assigned id + meta).
func (g SCIMGroup) ResponseGroup(assignedID, location string) SCIMGroup {
	out := g
	out.Schemas = []string{SCIMGroupSchema}
	out.ID = assignedID
	out.Meta = &SCIMMeta{ResourceType: "Group", Location: location}
	return out
}
