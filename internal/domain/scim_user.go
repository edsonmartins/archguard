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

// SCIM 2.0 inbound (pacote 009, RFC-0007 §5.2 / RFC 7643): ArchGuard is the
// provisioning TARGET of a customer IdP. This file is the protocol data model for
// the User resource — parse, validate, and map to the SAME neutral
// DirectorySyncRecord the LDAP connector produces, so both feed one provisioning
// path (dedup by email_hash downstream, T-009). No password is ever accepted here
// (RFC-0007 §4).

// SCIMUserSchema is the core User schema URN.
const SCIMUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"

// SCIMName is the SCIM name sub-attribute.
type SCIMName struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

// SCIMEmail is one SCIM e-mail value.
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMMeta is the SCIM common metadata.
type SCIMMeta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Location     string `json:"location,omitempty"`
}

// SCIMUser is the SCIM 2.0 User resource (the subset ArchGuard consumes).
type SCIMUser struct {
	Schemas    []string    `json:"schemas"`
	ID         string      `json:"id,omitempty"`
	ExternalID string      `json:"externalId,omitempty"`
	UserName   string      `json:"userName"`
	Name       SCIMName    `json:"name,omitempty"`
	Emails     []SCIMEmail `json:"emails,omitempty"`
	Active     bool        `json:"active"`
	Meta       *SCIMMeta   `json:"meta,omitempty"`
}

// Errors of SCIM user parsing/validation.
var (
	ErrSCIMMalformed     = errors.New("scim: corpo malformado")
	ErrSCIMSchema        = errors.New("scim: schema de User ausente")
	ErrSCIMUserName      = errors.New("scim: userName obrigatório")
	ErrSCIMEmailRequired = errors.New("scim: e-mail obrigatório (chave de deduplicação)")
)

// ParseSCIMUser decodes and validates a SCIM User payload. It requires the User
// schema, a userName, and an e-mail (the dedup key). It NEVER reads a password —
// SCIM password attributes, if present, are ignored (RFC-0007 §4).
func ParseSCIMUser(body []byte) (SCIMUser, error) {
	var u SCIMUser
	dec := json.NewDecoder(strings.NewReader(string(body)))
	if err := dec.Decode(&u); err != nil {
		return SCIMUser{}, fmt.Errorf("%w: %v", ErrSCIMMalformed, err)
	}
	if !containsSchema(u.Schemas, SCIMUserSchema) {
		return SCIMUser{}, ErrSCIMSchema
	}
	if strings.TrimSpace(u.UserName) == "" {
		return SCIMUser{}, ErrSCIMUserName
	}
	if u.PrimaryEmail() == "" {
		return SCIMUser{}, ErrSCIMEmailRequired
	}
	return u, nil
}

func containsSchema(schemas []string, want string) bool {
	for _, s := range schemas {
		if s == want {
			return true
		}
	}
	return false
}

// PrimaryEmail returns the primary e-mail, or the first e-mail if none is flagged
// primary, or "".
func (u SCIMUser) PrimaryEmail() string {
	for _, e := range u.Emails {
		if e.Primary && e.Value != "" {
			return e.Value
		}
	}
	for _, e := range u.Emails {
		if e.Value != "" {
			return e.Value
		}
	}
	return ""
}

// DisplayName returns the best available display name (formatted, else
// "given family", else userName).
func (u SCIMUser) DisplayName() string {
	if u.Name.Formatted != "" {
		return u.Name.Formatted
	}
	full := strings.TrimSpace(u.Name.GivenName + " " + u.Name.FamilyName)
	if full != "" {
		return full
	}
	return u.UserName
}

// ToSyncRecord maps the SCIM user to the neutral provisioning record — the same
// type the LDAP connector produces — so identity+membership reconciliation (dedup
// by email, suspension on Active=false) is shared across both sources. ExternalID
// prefers the IdP's externalId, falling back to userName.
func (u SCIMUser) ToSyncRecord() DirectorySyncRecord {
	external := u.ExternalID
	if external == "" {
		external = u.UserName
	}
	return DirectorySyncRecord{
		ExternalID: external,
		Email:      u.PrimaryEmail(),
		Attributes: map[string]string{"email": u.PrimaryEmail(), "name": u.DisplayName()},
		Active:     u.Active,
	}
}

// ResponseUser fills the resource for a SCIM response: it stamps the assigned id,
// the User schema, and meta. The password is never echoed (there is none).
func (u SCIMUser) ResponseUser(assignedID, location string) SCIMUser {
	out := u
	out.Schemas = []string{SCIMUserSchema}
	out.ID = assignedID
	out.Meta = &SCIMMeta{ResourceType: "User", Location: location}
	return out
}
