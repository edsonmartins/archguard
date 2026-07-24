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
	"strings"

	"github.com/google/uuid"
)

// DirectoryConnector is a tenant's configuration for syncing identities and groups
// from a corporate directory (LDAP/AD) — pacote 009, RFC-0007 §5.1. It is a PURE
// domain entity (INV-3): the LDAP client lives in the adapter (T-002).
//
// Two safety properties are baked into construction:
//
//   - A MANDATORY scope filter (RFC-0007 §5.1 / spec "Escopo não definido"):
//     syncing "the whole tree" is forbidden; a connector with no scope is rejected.
//   - Connector credentials are NEVER stored here — only CredentialRef, a pointer
//     to the secret custodied in the vault (OpenBao, ADR-0012 / INV-7). The secret
//     never touches the database or a log.
//
// The directory→ArchGuard mapping is VERSIONED: every change is a new version, so
// precedence and audit have a stable reference. Directory authority covers
// attributes and group membership; PRIVILEGED roles and grants are ALWAYS the
// ArchGuard's and are never auto-derived from a directory group without an
// explicitly approved mapping (design 009 / spec "Grupo sem mapeamento aprovado").
type DirectoryConnector struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Kind           DirectoryKind
	Name           string
	// ScopeFilter bounds what is synced (an LDAP filter / subtree). Mandatory.
	ScopeFilter string
	// CredentialRef points to the connector's bind credentials in the vault. It is
	// NOT the credential (INV-7).
	CredentialRef string
	// Enabled gates whether sync runs. A new connector starts disabled (safe
	// default): it is enabled explicitly after the mapping is reviewed.
	Enabled bool
	Mapping ConnectorMapping
}

// DirectoryKind is the directory technology of a connector.
type DirectoryKind string

const (
	DirectoryLDAP DirectoryKind = "ldap"
	DirectoryAD   DirectoryKind = "ad"
)

// Valid reports whether k is a defined directory kind.
func (k DirectoryKind) Valid() bool { return k == DirectoryLDAP || k == DirectoryAD }

// AttributeMapping maps one directory attribute to an ArchGuard attribute.
type AttributeMapping struct {
	DirectoryAttr string
	ArchGuardAttr string
}

// GroupMapping maps a directory group to an ArchGuard target group. Approved gates
// whether the mapping is active: an UNAPPROVED mapping grants nothing, so no
// privileged role is ever auto-derived from a directory group without explicit
// approval (design 009 / spec "Grupo sem mapeamento aprovado").
type GroupMapping struct {
	DirectoryGroup string
	TargetGroup    string
	Approved       bool
}

// ConnectorMapping is the versioned directory→ArchGuard mapping. Version starts at
// 1 and increments on every revision (ReviseMapping).
type ConnectorMapping struct {
	Version    int
	Attributes []AttributeMapping
	Groups     []GroupMapping
}

// Errors of directory-connector construction.
var (
	ErrInvalidConnector = errors.New("directory_connector: dados obrigatórios ausentes")
	// ErrScopeFilterRequired is returned when a connector is configured without a
	// scope filter — syncing the whole tree is forbidden (RFC-0007 §5.1).
	ErrScopeFilterRequired = errors.New("directory_connector: filtro de escopo obrigatório (sincronizar toda a árvore é proibido)")
	// ErrCredentialRefRequired is returned when no vault reference for the connector
	// credentials is given (the secret is custodied, never inlined).
	ErrCredentialRefRequired = errors.New("directory_connector: referência de credencial no cofre obrigatória")
	// ErrInvalidMapping is returned when a mapping entry is incomplete.
	ErrInvalidMapping = errors.New("directory_connector: mapeamento inválido")
	// ErrScopeFilterTooBroad is returned when the scope filter matches the whole
	// subtree (a match-all), which defeats the mandatory scoping (RFC-0007 §5.1).
	ErrScopeFilterTooBroad = errors.New("directory_connector: filtro de escopo abrangente demais (equivale a sincronizar toda a árvore)")
	// ErrScopeFilterMalformed is returned when the scope filter is not a
	// well-formed LDAP filter (unbalanced parentheses).
	ErrScopeFilterMalformed = errors.New("directory_connector: filtro de escopo mal-formado")
)

// ValidateScopeFilter enforces that a connector's scope is DELIBERATE and BOUNDED
// (RFC-0007 §5.1 / spec "Escopo não definido"). It rejects: an empty filter; a
// match-all filter ("(objectClass=*)", "*", …), which is "the whole tree" in
// disguise; and a filter with unbalanced parentheses, which the directory would
// misinterpret. It does not attempt full LDAP-filter parsing — only the config-time
// guards that keep an operator from accidentally syncing everything.
func ValidateScopeFilter(filter string) error {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return ErrScopeFilterRequired
	}
	if isMatchAllFilter(trimmed) {
		return ErrScopeFilterTooBroad
	}
	if !balancedParens(trimmed) {
		return ErrScopeFilterMalformed
	}
	return nil
}

// isMatchAllFilter reports whether the filter matches every entry in the subtree.
func isMatchAllFilter(filter string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(filter, " ", ""))
	switch normalized {
	case "*", "(*)", "objectclass=*", "(objectclass=*)":
		return true
	default:
		return false
	}
}

// balancedParens reports whether parentheses are balanced and never close before
// they open — a cheap well-formedness check for an LDAP filter.
func balancedParens(filter string) bool {
	depth := 0
	for _, r := range filter {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// NewDirectoryConnector builds a validated connector (UUIDv7 id), starting
// DISABLED with mapping version 1. A missing scope filter or credential reference
// is refused. attrs/groups may be empty at creation and revised later.
func NewDirectoryConnector(organizationID uuid.UUID, kind DirectoryKind, name, scopeFilter, credentialRef string, attrs []AttributeMapping, groups []GroupMapping) (DirectoryConnector, error) {
	if organizationID == uuid.Nil {
		return DirectoryConnector{}, fmt.Errorf("%w: organização", ErrInvalidConnector)
	}
	if !kind.Valid() {
		return DirectoryConnector{}, fmt.Errorf("%w: tipo %q", ErrInvalidConnector, kind)
	}
	if name == "" {
		return DirectoryConnector{}, fmt.Errorf("%w: nome", ErrInvalidConnector)
	}
	if err := ValidateScopeFilter(scopeFilter); err != nil {
		return DirectoryConnector{}, err
	}
	if credentialRef == "" {
		return DirectoryConnector{}, ErrCredentialRefRequired
	}
	mapping := ConnectorMapping{Version: 1, Attributes: attrs, Groups: groups}
	if err := mapping.validate(); err != nil {
		return DirectoryConnector{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return DirectoryConnector{}, fmt.Errorf("directory_connector: geração de UUIDv7 falhou: %w", err)
	}
	return DirectoryConnector{
		ID:             id,
		OrganizationID: organizationID,
		Kind:           kind,
		Name:           name,
		ScopeFilter:    scopeFilter,
		CredentialRef:  credentialRef,
		Enabled:        false,
		Mapping:        mapping,
	}, nil
}

// validate checks that every mapping entry is complete.
func (m ConnectorMapping) validate() error {
	if m.Version < 1 {
		return fmt.Errorf("%w: versão deve ser >= 1", ErrInvalidMapping)
	}
	for _, a := range m.Attributes {
		if a.DirectoryAttr == "" || a.ArchGuardAttr == "" {
			return fmt.Errorf("%w: atributo com lado vazio", ErrInvalidMapping)
		}
	}
	for _, g := range m.Groups {
		if g.DirectoryGroup == "" || g.TargetGroup == "" {
			return fmt.Errorf("%w: grupo com lado vazio", ErrInvalidMapping)
		}
	}
	return nil
}

// ReviseMapping replaces the mapping with a new version (current + 1), validating
// it first. Versioning makes each change an auditable, referenceable state.
func (c *DirectoryConnector) ReviseMapping(attrs []AttributeMapping, groups []GroupMapping) error {
	next := ConnectorMapping{Version: c.Mapping.Version + 1, Attributes: attrs, Groups: groups}
	if err := next.validate(); err != nil {
		return err
	}
	c.Mapping = next
	return nil
}

// Enable/Disable toggle whether sync runs for this connector.
func (c *DirectoryConnector) Enable()  { c.Enabled = true }
func (c *DirectoryConnector) Disable() { c.Enabled = false }

// ApprovedGroupTarget returns the target group a directory group maps to, and
// whether such an APPROVED mapping exists. An unapproved (or absent) mapping
// returns ok=false — the caller grants nothing, so a directory group never confers
// access without approval.
func (c DirectoryConnector) ApprovedGroupTarget(directoryGroup string) (target string, ok bool) {
	for _, g := range c.Mapping.Groups {
		if g.DirectoryGroup == directoryGroup && g.Approved {
			return g.TargetGroup, true
		}
	}
	return "", false
}
