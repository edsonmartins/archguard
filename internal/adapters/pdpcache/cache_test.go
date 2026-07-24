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

package pdpcache

import (
	"context"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// countingPDP counts how many times each method reaches the underlying PDP.
type countingPDP struct {
	checks int
	lists  int
	writes int
}

func (p *countingPDP) Check(context.Context, domain.CheckRequest) (domain.Decision, error) {
	p.checks++
	return domain.Allow("ok"), nil
}
func (p *countingPDP) ListObjects(context.Context, domain.ListObjectsRequest) ([]string, error) {
	p.lists++
	return []string{"org:o1/asset:a1"}, nil
}
func (p *countingPDP) Write(context.Context, []domain.TupleUpdate) error { p.writes++; return nil }
func (p *countingPDP) Read(context.Context, domain.TupleFilter) ([]domain.RelationTuple, error) {
	return nil, nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func listReq() domain.ListObjectsRequest {
	return domain.ListObjectsRequest{User: "org:o1/membership:m1", Relation: domain.RelOperator, Type: "org:o1/asset"}
}

// Listagem: a segunda chamada dentro do TTL vem do cache (não toca o PDP).
func TestListObjectsCachedWithinTTL(t *testing.T) {
	inner := &countingPDP{}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := New(inner, DefaultTTL, clk.now)

	if _, err := c.ListObjects(context.Background(), listReq()); err != nil {
		t.Fatalf("list1: %v", err)
	}
	if _, err := c.ListObjects(context.Background(), listReq()); err != nil {
		t.Fatalf("list2: %v", err)
	}
	if inner.lists != 1 {
		t.Fatalf("segunda listagem deveria vir do cache; PDP tocado %d vezes", inner.lists)
	}
}

// Após o TTL, a listagem reconsulta o PDP.
func TestListObjectsRefetchesAfterTTL(t *testing.T) {
	inner := &countingPDP{}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := New(inner, DefaultTTL, clk.now)

	_, _ = c.ListObjects(context.Background(), listReq())
	clk.advance(DefaultTTL + time.Second)
	_, _ = c.ListObjects(context.Background(), listReq())
	if inner.lists != 2 {
		t.Fatalf("após expirar o TTL deveria reconsultar; tocado %d vezes", inner.lists)
	}
}

// A decisão privilegiada (Check) NUNCA é cacheada: toda chamada toca o PDP.
func TestCheckNeverCached(t *testing.T) {
	inner := &countingPDP{}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := New(inner, DefaultTTL, clk.now)

	req := domain.CheckRequest{Tuple: domain.RelationTuple{
		User: "org:o1/membership:m1", Relation: domain.RelCanOpenPrivilegedSession, Object: "org:o1/asset:a1"}}
	for i := 0; i < 3; i++ {
		if _, err := c.Check(context.Background(), req); err != nil {
			t.Fatalf("check: %v", err)
		}
	}
	if inner.checks != 3 {
		t.Fatalf("Check nunca deveria ser cacheado; esperava 3 chamadas, veio %d", inner.checks)
	}
}

// Um write invalida o cache de listagens.
func TestWriteFlushesListingCache(t *testing.T) {
	inner := &countingPDP{}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := New(inner, DefaultTTL, clk.now)

	_, _ = c.ListObjects(context.Background(), listReq())
	if err := c.Write(context.Background(), nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _ = c.ListObjects(context.Background(), listReq())
	if inner.lists != 2 {
		t.Fatalf("write deveria invalidar o cache; tocado %d vezes", inner.lists)
	}
}

// TTL não positivo desliga o cache (toda listagem passa direto).
func TestZeroTTLDisablesCache(t *testing.T) {
	inner := &countingPDP{}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := New(inner, 0, clk.now)

	_, _ = c.ListObjects(context.Background(), listReq())
	_, _ = c.ListObjects(context.Background(), listReq())
	if inner.lists != 2 {
		t.Fatalf("com TTL zero não deveria cachear; tocado %d vezes", inner.lists)
	}
}
