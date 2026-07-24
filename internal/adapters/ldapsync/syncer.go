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

// Package ldapsync implements the LDAP/AD directory connector (pacote 009, T-002):
// an INCREMENTAL search bounded by the connector's mandatory scope filter, mapping
// directory entries to protocol-neutral domain.DirectorySyncRecord values. It uses
// the go-ldap client already in the tree (MIT). The connection (host, base DN, TLS,
// bind credentials from the vault) is supplied by the caller — this type only
// searches an already-bound connection, so the secret never lives here (INV-7).
package ldapsync

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/go-ldap/ldap/v3"
)

// Searcher is the slice of the go-ldap connection the syncer needs; *ldap.Conn
// satisfies it, and a fake satisfies it in tests (no live directory required).
type Searcher interface {
	Search(req *ldap.SearchRequest) (*ldap.SearchResult, error)
}

// SyncResult is the outcome of one incremental sync pass.
type SyncResult struct {
	// Records are the mapped directory entries in scope.
	Records []domain.DirectorySyncRecord
	// HighWater is the greatest modifyTimestamp seen; persist it as the next sync's
	// `since` so the following pass fetches only what changed after it.
	HighWater time.Time
}

// ldapTimeLayout is the LDAP GeneralizedTime layout (UTC, "Z").
const ldapTimeLayout = "20060102150405Z"

// groupAttr / modifyAttr are the operational attributes the sync always reads.
const (
	groupAttr  = "memberOf"
	modifyAttr = "modifyTimestamp"
	uacAttr    = "userAccountControl" // AD account flags (ACCOUNTDISABLE = 0x2)
)

// Syncer performs incremental directory syncs for a connector.
type Syncer struct{}

// NewSyncer builds the syncer.
func NewSyncer() *Syncer { return &Syncer{} }

// Sync runs one INCREMENTAL search over searcher, bounded by the connector's
// mandatory scope filter and (when `since` is non-zero) by modifyTimestamp >=
// since, so only entries changed after the last high-water mark are returned. A
// zero `since` is a full initial sync. searchBase is the subtree root (connection
// config). Entries are mapped through the connector's attribute mapping; groups
// come from memberOf; Active is derived per directory kind. Returns the records
// and the new high-water mark.
func (s *Syncer) Sync(_ context.Context, searcher Searcher, searchBase string, c domain.DirectoryConnector, since time.Time) (SyncResult, error) {
	if searchBase == "" {
		return SyncResult{}, fmt.Errorf("ldapsync: base de busca obrigatória")
	}
	if c.ScopeFilter == "" {
		// Defense in depth — the domain already forbids this, but never search the
		// whole tree by accident (RFC-0007 §5.1).
		return SyncResult{}, domain.ErrScopeFilterRequired
	}

	filter := c.ScopeFilter
	if !since.IsZero() {
		filter = "(&" + c.ScopeFilter + "(" + modifyAttr + ">=" + since.UTC().Format(ldapTimeLayout) + "))"
	}

	req := ldap.NewSearchRequest(
		searchBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter, s.attributes(c), nil,
	)
	res, err := searcher.Search(req)
	if err != nil {
		return SyncResult{}, fmt.Errorf("ldapsync: busca no diretório falhou: %w", err)
	}

	idAttr := externalIDAttr(c.Kind)
	var result SyncResult
	for _, entry := range res.Entries {
		raw := rawAttributes(entry, c)
		attrs := domain.ApplyAttributeMapping(c.Mapping.Attributes, raw)
		rec := domain.DirectorySyncRecord{
			ExternalID: firstNonEmpty(entry.GetAttributeValue(idAttr), entry.DN),
			Email:      domain.MappedEmail(attrs),
			Attributes: attrs,
			Groups:     entry.GetAttributeValues(groupAttr),
			Active:     activeFromEntry(c.Kind, entry.GetAttributeValue(uacAttr)),
		}
		result.Records = append(result.Records, rec)

		if ts, err := time.Parse(ldapTimeLayout, entry.GetAttributeValue(modifyAttr)); err == nil {
			if ts.After(result.HighWater) {
				result.HighWater = ts
			}
		}
	}
	return result, nil
}

// attributes is the set the search requests: the mapped directory attributes plus
// the operational ones (group, modify time, id, and AD flags).
func (s *Syncer) attributes(c domain.DirectoryConnector) []string {
	seen := map[string]bool{}
	var attrs []string
	add := func(a string) {
		if a != "" && !seen[a] {
			seen[a] = true
			attrs = append(attrs, a)
		}
	}
	for _, m := range c.Mapping.Attributes {
		add(m.DirectoryAttr)
	}
	add(groupAttr)
	add(modifyAttr)
	add(externalIDAttr(c.Kind))
	if c.Kind == domain.DirectoryAD {
		add(uacAttr)
	}
	return attrs
}

// rawAttributes flattens the entry's mapped directory attributes to single values
// (first value), the shape ApplyAttributeMapping consumes.
func rawAttributes(entry *ldap.Entry, c domain.DirectoryConnector) map[string]string {
	raw := make(map[string]string, len(c.Mapping.Attributes))
	for _, m := range c.Mapping.Attributes {
		if v := entry.GetAttributeValue(m.DirectoryAttr); v != "" {
			raw[m.DirectoryAttr] = v
		}
	}
	return raw
}

// externalIDAttr is the conventional stable-id attribute per directory kind.
func externalIDAttr(kind domain.DirectoryKind) string {
	if kind == domain.DirectoryAD {
		return "objectGUID"
	}
	return "entryUUID"
}

// activeFromEntry decides whether an entry is active. On AD, the ACCOUNTDISABLE bit
// (0x2) of userAccountControl marks a disabled account; on plain LDAP, an entry in
// scope is active (a directory-specific disabled attribute can refine this later).
func activeFromEntry(kind domain.DirectoryKind, uac string) bool {
	if kind == domain.DirectoryAD {
		n, err := strconv.Atoi(uac)
		if err != nil {
			// No/invalid flags → treat as active; the reconciliation is conservative
			// about SUSPENDING, never about granting.
			return true
		}
		return n&0x2 == 0
	}
	return true
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
