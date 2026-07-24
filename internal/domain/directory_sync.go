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

// DirectorySyncRecord is one directory entry projected onto ArchGuard's model by a
// sync (pacote 009). It is the neutral, protocol-free result the LDAP/AD adapter
// (T-002) or SCIM (T-007) produces; the reconciliation into identity + membership
// (dedup by email_hash, suspension on deactivation) consumes it (T-005/T-012).
//
// It carries NO password (RFC-0007 §4: no credential is ever imported) and only
// the attributes the connector's mapping explicitly selects.
type DirectorySyncRecord struct {
	// ExternalID is the directory's stable unique id for the entry (objectGUID on
	// AD, entryUUID on LDAP) — the anchor for incremental sync, independent of a
	// renamed DN.
	ExternalID string
	// Email is the mapped e-mail, the dedup key (hashed to email_hash downstream,
	// RFC-0002 §6). Empty if the directory entry has none.
	Email string
	// Attributes are the mapped ArchGuard attributes (archGuardAttr → value).
	Attributes map[string]string
	// Groups are the directory groups the entry belongs to (DNs/names), matched
	// against the connector's APPROVED group mappings downstream.
	Groups []string
	// Active is whether the entry is active in the directory. A deactivation
	// (Active=false) suspends the membership — never deletes (spec
	// "Desprovisionamento reflete o diretório").
	Active bool
}

// ApplyAttributeMapping projects a raw directory entry (directoryAttr → value)
// onto ArchGuard attributes using the connector's mapping. Only mapped attributes
// present in the entry are carried — nothing outside the explicit mapping leaks
// through. It is pure, so the mapping is testable without a directory.
func ApplyAttributeMapping(mapping []AttributeMapping, raw map[string]string) map[string]string {
	out := make(map[string]string, len(mapping))
	for _, m := range mapping {
		if v, ok := raw[m.DirectoryAttr]; ok && v != "" {
			out[m.ArchGuardAttr] = v
		}
	}
	return out
}

// MappedEmail returns the value mapped to the ArchGuard "email" attribute, if the
// connector maps one and the entry carries it.
func MappedEmail(attrs map[string]string) string {
	return attrs["email"]
}
