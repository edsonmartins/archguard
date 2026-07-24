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

// Package pdpcache adds a VERY SHORT-lived cache in front of a
// domain.PolicyDecisionPoint — but only for listings (RFC-0004 §5). The privileged
// decision (Check) is NEVER cached: it always reaches the underlying PDP so a
// session is opened on a fresh verdict, honoring "decisões de abertura de sessão
// privilegiada nunca usam cache". Listings (ListObjects, used by access-review
// campaigns) tolerate a few seconds of staleness in exchange for not re-walking
// the graph per row of a report.
package pdpcache

import (
	"context"
	"sync"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// DefaultTTL is the short listing cache lifetime. It is deliberately tiny: a
// listing may be a few seconds stale, an access DECISION never is (Check bypasses
// this cache entirely).
const DefaultTTL = 3 * time.Second

// CachingPDP wraps a PolicyDecisionPoint, caching ONLY ListObjects results for a
// short TTL. Check, Read and Write pass straight through; a Write also flushes the
// listing cache, since a projection change can alter what a listing returns.
type CachingPDP struct {
	inner domain.PolicyDecisionPoint
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	objects   []string
	expiresAt time.Time
}

// New wraps inner with a listing cache of the given TTL. now supplies the clock
// (injected so the domain stays clock-free and tests are deterministic); pass
// time.Now in production. A non-positive ttl disables caching (every call passes
// through).
func New(inner domain.PolicyDecisionPoint, ttl time.Duration, now func() time.Time) *CachingPDP {
	return &CachingPDP{inner: inner, ttl: ttl, now: now, entries: map[string]cacheEntry{}}
}

// Check is NEVER cached — it always reaches the underlying PDP (privileged decision
// freshness, RFC-0004 §5).
func (c *CachingPDP) Check(ctx context.Context, req domain.CheckRequest) (domain.Decision, error) {
	return c.inner.Check(ctx, req)
}

// ListObjects serves from the short cache when a fresh entry exists, otherwise
// queries the inner PDP and caches the result. An error is never cached. A cached
// list is returned as a copy so a caller cannot mutate the cache.
func (c *CachingPDP) ListObjects(ctx context.Context, req domain.ListObjectsRequest) ([]string, error) {
	if c.ttl <= 0 {
		return c.inner.ListObjects(ctx, req)
	}
	key := req.User + "\x00" + req.Relation + "\x00" + req.Type
	now := c.now()

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Before(e.expiresAt) {
		out := append([]string(nil), e.objects...)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	objects, err := c.inner.ListObjects(ctx, req)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry{objects: append([]string(nil), objects...), expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()
	return objects, nil
}

// Write passes through and flushes the listing cache (a projection change can alter
// listings; the decision path is unaffected because Check is never cached).
func (c *CachingPDP) Write(ctx context.Context, updates []domain.TupleUpdate) error {
	if err := c.inner.Write(ctx, updates); err != nil {
		return err
	}
	c.mu.Lock()
	c.entries = map[string]cacheEntry{}
	c.mu.Unlock()
	return nil
}

// Read passes straight through (not a listing; used by reconciliation/debug).
func (c *CachingPDP) Read(ctx context.Context, filter domain.TupleFilter) ([]domain.RelationTuple, error) {
	return c.inner.Read(ctx, filter)
}

var _ domain.PolicyDecisionPoint = (*CachingPDP)(nil)
