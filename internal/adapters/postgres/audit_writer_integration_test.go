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
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func fixedClock() Clock {
	ts := time.Unix(1_700_000_000, 123_456_000).UTC()
	return func() time.Time { return ts }
}

// cleanupAudit removes an organization's trail rows at test end. audit_event is
// append-only (blocked by the T-007 triggers even for the superuser), so the
// cleanup uses the documented superuser bypass (session_replication_role =
// replica) on a single pinned connection — test hygiene only; the legitimate
// removal path is archiving (T-018).
func cleanupAudit(t *testing.T, pool *pgxpool.Pool, orgs ...uuid.UUID) {
	t.Cleanup(func() {
		bg := context.Background()
		conn, err := pool.Acquire(bg)
		if err != nil {
			return
		}
		defer conn.Release()
		if _, err := conn.Exec(bg, "SET session_replication_role = replica"); err != nil {
			return
		}
		for _, o := range orgs {
			_, _ = conn.Exec(bg, "DELETE FROM audit_event WHERE organization_id = $1", o.String())
			_, _ = conn.Exec(bg, "DELETE FROM audit_seal WHERE organization_id = $1", o.String())
			_, _ = conn.Exec(bg, "DELETE FROM audit_event_queue WHERE organization_id = $1", o.String())
			_, _ = conn.Exec(bg, "DELETE FROM audit_chain_head WHERE organization_id = $1", o.String())
		}
		_, _ = conn.Exec(bg, "SET session_replication_role = origin")
	})
}

// chainRow is the stored shape needed to check chaining structurally.
type chainRow struct {
	seq      int64
	prevHash []byte
	hash     []byte
}

func readChain(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) []chainRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT seq, prev_hash, hash FROM audit_event WHERE organization_id = $1 ORDER BY seq", orgID.String())
	if err != nil {
		t.Fatalf("leitura da cadeia: %v", err)
	}
	defer rows.Close()
	var out []chainRow
	for rows.Next() {
		var r chainRow
		if err := rows.Scan(&r.seq, &r.prevHash, &r.hash); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iteração: %v", err)
	}
	return out
}

func chainGenesis(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) []byte {
	t.Helper()
	var nonce []byte
	if err := pool.QueryRow(context.Background(),
		"SELECT genesis_nonce FROM audit_chain_head WHERE organization_id = $1", orgID.String()).Scan(&nonce); err != nil {
		t.Fatalf("genesis_nonce: %v", err)
	}
	g, err := domain.GenesisHash(orgID, nonce)
	if err != nil {
		t.Fatalf("GenesisHash: %v", err)
	}
	return g
}

// verifyChain checks the structural chain: first prev_hash is the genesis, each
// prev_hash is the previous event's hash, and seq is 1..n gapless.
func verifyChain(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, want int) {
	t.Helper()
	chain := readChain(t, pool, orgID)
	if len(chain) != want {
		t.Fatalf("eventos = %d, quero %d", len(chain), want)
	}
	prev := chainGenesis(t, pool, orgID)
	for i, r := range chain {
		if r.seq != int64(i+1) {
			t.Fatalf("seq no índice %d = %d, quero %d (lacuna/duplicata)", i, r.seq, i+1)
		}
		if !bytes.Equal(r.prevHash, prev) {
			t.Fatalf("elo %d: prev_hash não aponta para o hash anterior", r.seq)
		}
		prev = r.hash
	}
}

func minimalInput(orgID uuid.UUID) domain.AuditEventInput {
	return domain.AuditEventInput{
		OrganizationID: orgID,
		Action:         domain.ActionAuthLogin,
		Actor:          domain.AuditActor{IdentitySubject: "sub-writer"},
		Outcome:        domain.Allowed,
	}
}

// Append persiste o evento, atribui seq=1, encadeia a partir da gênese e avança
// o cabeçalho; o hash armazenado é o do domínio (recomputável).
func TestAuditWriterAppendAndChain(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)

	w := NewAuditWriter(pool, fixedClock())
	s1, err := w.Append(ctx, minimalInput(org))
	if err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if s1.Seq != 1 {
		t.Fatalf("seq = %d, quero 1", s1.Seq)
	}
	genesis := chainGenesis(t, pool, org)
	if !bytes.Equal(s1.PrevHash, genesis) {
		t.Fatalf("primeiro prev_hash não é a gênese")
	}

	// O hash é recomputável: reconstrói o evento das colunas e re-sela.
	var (
		eid, action, outcome, subj string
		occurredAt                 time.Time
		sv                         int
		storedHash, storedPrev     []byte
	)
	if err := pool.QueryRow(ctx,
		`SELECT event_id::text, occurred_at, schema_version, action, outcome, actor_subject, prev_hash, hash
		 FROM audit_event WHERE organization_id = $1 AND seq = 1`, org.String()).
		Scan(&eid, &occurredAt, &sv, &action, &outcome, &subj, &storedPrev, &storedHash); err != nil {
		t.Fatalf("leitura do evento: %v", err)
	}
	recomputed := domain.AuditEvent{
		SchemaVersion: sv, EventID: uuid.MustParse(eid), OrganizationID: org,
		Action: domain.Action(action), Actor: domain.AuditActor{IdentitySubject: subj},
		Outcome: domain.Allowed, OccurredAt: occurredAt,
	}
	reSealed, err := domain.SealEvent(recomputed, storedPrev, 1)
	if err != nil {
		t.Fatalf("SealEvent recomputado: %v", err)
	}
	if !bytes.Equal(reSealed.Hash, storedHash) {
		t.Fatalf("hash armazenado não recomputa a partir das colunas")
	}

	// Segundo evento: encadeia e avança.
	s2, err := w.Append(ctx, minimalInput(org))
	if err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	if s2.Seq != 2 || !bytes.Equal(s2.PrevHash, s1.Hash) {
		t.Fatalf("segundo elo errado: seq=%d", s2.Seq)
	}

	// O cabeçalho reflete o último evento.
	var headSeq int64
	var headHash []byte
	if err := pool.QueryRow(ctx,
		"SELECT last_seq, head_hash FROM audit_chain_head WHERE organization_id = $1", org.String()).
		Scan(&headSeq, &headHash); err != nil {
		t.Fatalf("cabeçalho: %v", err)
	}
	if headSeq != 2 || !bytes.Equal(headHash, s2.Hash) {
		t.Fatalf("cabeçalho não avançou para o último evento")
	}

	verifyChain(t, pool, org, 2)
}

// Cadeias por organização são independentes: cada org começa em seq 1.
func TestAuditWriterPerOrgChains(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	cleanupAudit(t, pool, orgA, orgB)
	w := NewAuditWriter(pool, fixedClock())

	a1, _ := w.Append(ctx, minimalInput(orgA))
	b1, _ := w.Append(ctx, minimalInput(orgB))
	if a1.Seq != 1 || b1.Seq != 1 {
		t.Fatalf("cada org deveria começar em seq 1: A=%d B=%d", a1.Seq, b1.Seq)
	}
	// As gêneses diferem (nonces distintos), logo os hashes também.
	if bytes.Equal(a1.Hash, b1.Hash) {
		t.Fatalf("orgs distintas não deveriam produzir o mesmo hash")
	}
	verifyChain(t, pool, orgA, 1)
	verifyChain(t, pool, orgB, 1)
}

// Cenário "Gravações concorrentes": N escritas simultâneas na MESMA org recebem
// seq distintos e consecutivos, e a cadeia permanece verificável.
func TestAuditWriterConcurrentSameOrg(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	w := NewAuditWriter(pool, fixedClock())

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.Append(ctx, minimalInput(org)); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Append concorrente: %v", err)
	}

	// seq 1..n sem lacuna nem duplicata, e cadeia encadeada corretamente.
	verifyChain(t, pool, org, n)
}

// Fail-closed atômico (T-008): AppendTx grava o evento NA transação do chamador.
// Se a transação da operação de negócio dá rollback, o evento some junto — nunca
// um evento de uma operação que não aconteceu, nem operação sem evento.
func TestAuditWriterAppendTxAtomic(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	w := NewAuditWriter(pool, fixedClock())

	// Rollback: AppendTx dentro de uma tx que falha depois → nada persistido.
	boom := errors.New("operação de negócio falhou após a auditoria")
	err := WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := w.AppendTx(ctx, tx, minimalInput(org)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("esperava o erro forçado, veio %v", err)
	}
	// Nada persistido: nem evento nem cabeçalho de cadeia (a criação lazy do
	// head na 1ª escrita também foi desfeita pelo rollback).
	if got := len(readChain(t, pool, org)); got != 0 {
		t.Fatalf("rollback deveria descartar o evento, restaram %d", got)
	}
	var heads int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM audit_chain_head WHERE organization_id = $1", org.String()).Scan(&heads); err != nil {
		t.Fatalf("consulta cabeçalho: %v", err)
	}
	if heads != 0 {
		t.Fatalf("cabeçalho não deveria existir após rollback, veio %d", heads)
	}

	// Commit: AppendTx numa tx que conclui → evento durável.
	if err := WithTx(ctx, pool, func(tx pgx.Tx) error {
		_, e := w.AppendTx(ctx, tx, minimalInput(org))
		return e
	}); err != nil {
		t.Fatalf("AppendTx commit: %v", err)
	}
	verifyChain(t, pool, org, 1)

	// E o writer satisfaz o porto domain.AuditSink.
	var _ domain.AuditSink = w
}
