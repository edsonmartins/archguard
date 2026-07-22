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

package postgres

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// auditBudget is the latency ceiling for the SYNCHRONOUS audit write on the hot
// path. RFC-0001 budgets token emission at p95 < 150 ms; the audit append is
// only a fraction of a login, so we require its p95 to stay well within that.
// Generous enough not to flake on a loaded CI, tight enough to catch a
// pathological regression (a missing index, a lock storm).
const auditBudget = 60 * time.Millisecond

// Mede o impacto da auditoria síncrona na latência de login: cada login paga um
// Append (auth.login). Medimos p50/p95 do Append e exigimos folga no orçamento.
func TestAuditSyncLatencyBudget(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	w := NewAuditWriter(pool, time.Now)

	const n = 300
	durations := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		if _, err := w.Append(ctx, loginInput(org)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[len(durations)*50/100]
	p95 := durations[len(durations)*95/100]
	t.Logf("Append síncrono sobre %d eventos: p50=%v p95=%v max=%v", n, p50, p95, durations[len(durations)-1])

	if p95 > auditBudget {
		t.Fatalf("p95 do Append síncrono = %v excede o orçamento %v (RFC-0001)", p95, auditBudget)
	}
	// Cadeia íntegra após a carga (sem lacuna sob volume).
	verifyChain(t, pool, org, n)
}

// Throughput concorrente: orgs distintas escrevem em PARALELO (sem contenção do
// cabeçalho); mesma org SERIALIZA (o FOR UPDATE), mantendo a cadeia sem lacuna.
func TestAuditConcurrentThroughput(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	w := NewAuditWriter(pool, time.Now)

	// 8 orgs em paralelo, 50 eventos cada.
	const orgs, per = 8, 50
	ids := make([]uuid.UUID, orgs)
	for i := range ids {
		ids[i] = uuid.New()
	}
	cleanupAudit(t, pool, ids...)

	start := time.Now()
	var wg sync.WaitGroup
	errs := make(chan error, orgs)
	for _, org := range ids {
		wg.Add(1)
		go func(org uuid.UUID) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				if _, err := w.Append(ctx, loginInput(org)); err != nil {
					errs <- err
					return
				}
			}
		}(org)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Append concorrente: %v", err)
	}
	elapsed := time.Since(start)
	total := orgs * per
	t.Logf("throughput: %d eventos (%d orgs x %d) em %v = %.0f eventos/s",
		total, orgs, per, elapsed, float64(total)/elapsed.Seconds())

	// Cada org manteve a cadeia sem lacuna (a serialização por tenant valeu).
	for _, org := range ids {
		verifyChain(t, pool, org, per)
	}
}

// BenchmarkAuditAppend mede o custo do Append síncrono (go test -bench).
func BenchmarkAuditAppend(b *testing.B) {
	pool := setupTenantPool(b)
	ctx := context.Background()
	org := uuid.New()
	w := NewAuditWriter(pool, time.Now)
	b.Cleanup(func() {
		bg := context.Background()
		conn, err := pool.Acquire(bg)
		if err == nil {
			defer conn.Release()
			_, _ = conn.Exec(bg, "SET session_replication_role = replica")
			_, _ = conn.Exec(bg, "DELETE FROM audit_event WHERE organization_id = $1", org.String())
			_, _ = conn.Exec(bg, "DELETE FROM audit_chain_head WHERE organization_id = $1", org.String())
			_, _ = conn.Exec(bg, "SET session_replication_role = origin")
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := w.Append(ctx, loginInput(org)); err != nil {
			b.Fatalf("Append: %v", err)
		}
	}
}

func loginInput(org uuid.UUID) domain.AuditEventInput {
	return domain.AuditEventInput{
		OrganizationID: org,
		Action:         domain.ActionAuthLogin,
		Actor:          domain.AuditActor{IdentitySubject: "sub-load"},
		Outcome:        domain.Allowed,
		Context:        domain.AuditContext{IP: "203.0.113.9", UserAgent: "load/1.0", AuthContextClass: "L1", AuthMethods: []string{"pwd"}},
	}
}
