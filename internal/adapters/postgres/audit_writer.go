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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Clock supplies the trusted time source that stamps occurred_at (RFC-0003 §2).
// Injected so tests are deterministic; production passes time.Now.
type Clock func() time.Time

// AuditWriter appends events to the immutable trail (RFC-0003 §3), serializing
// per organization so the chain is gapless and race-free. Each append runs in
// one transaction that locks the organization's chain-head row (SELECT ... FOR
// UPDATE): concurrent writes to the SAME organization take the lock in turn and
// receive distinct consecutive seq; writes to different organizations proceed in
// parallel. The chain hashing itself is the domain's (SealEvent, T-003); this
// adapter only assigns seq/prev_hash under the lock and persists.
type AuditWriter struct {
	db    Beginner
	clock Clock
}

// NewAuditWriter builds the writer. A nil clock defaults to time.Now.
func NewAuditWriter(db Beginner, clock Clock) *AuditWriter {
	if clock == nil {
		clock = time.Now
	}
	return &AuditWriter{db: db, clock: clock}
}

// Append validates and persists one event, returning the sealed event (with its
// assigned seq and chain hash). occurred_at is stamped from the trusted clock.
// The whole operation is one transaction (RFC-0002 §5); on any failure nothing
// is written and the chain head is unchanged — which is exactly what the
// fail-closed sink (T-008) needs to deny the operation.
func (w *AuditWriter) Append(ctx context.Context, in domain.AuditEventInput) (domain.SealedEvent, error) {
	event, err := domain.NewAuditEvent(in)
	if err != nil {
		return domain.SealedEvent{}, err
	}
	event.OccurredAt = w.clock().UTC()

	var sealed domain.SealedEvent
	err = WithTx(ctx, w.db, func(tx pgx.Tx) error {
		prevHash, lastSeq, err := lockChainHead(ctx, tx, event.OrganizationID)
		if err != nil {
			return err
		}
		sealed, err = domain.SealEvent(event, prevHash, lastSeq+1)
		if err != nil {
			return err
		}
		if err := insertAuditEvent(ctx, tx, sealed); err != nil {
			return err
		}
		return advanceChainHead(ctx, tx, event.OrganizationID, sealed.Seq, sealed.Hash)
	})
	if err != nil {
		return domain.SealedEvent{}, err
	}
	return sealed, nil
}

// lockChainHead returns the organization's current head hash and last seq,
// holding a row lock for the rest of the transaction. On the organization's
// first ever event the head row does not exist yet: it is created with a fresh
// random genesis nonce and head_hash = GenesisHash(org, nonce). The
// INSERT ... ON CONFLICT DO NOTHING makes the create race-safe (a concurrent
// first write loses the insert and reads the winner's row under the lock).
func lockChainHead(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (prevHash []byte, lastSeq int64, err error) {
	nonce := make([]byte, domain.AuditGenesisNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, fmt.Errorf("postgres: geração do genesis_nonce falhou: %w", err)
	}
	genesis, err := domain.GenesisHash(orgID, nonce)
	if err != nil {
		return nil, 0, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_chain_head (organization_id, last_seq, head_hash, genesis_nonce)
		 VALUES ($1, 0, $2, $3) ON CONFLICT (organization_id) DO NOTHING`,
		orgID.String(), genesis, nonce); err != nil {
		return nil, 0, fmt.Errorf("postgres: criação do cabeçalho de cadeia falhou: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT head_hash, last_seq FROM audit_chain_head WHERE organization_id = $1 FOR UPDATE`,
		orgID.String()).Scan(&prevHash, &lastSeq); err != nil {
		return nil, 0, fmt.Errorf("postgres: trava do cabeçalho de cadeia falhou: %w", err)
	}
	return prevHash, lastSeq, nil
}

// advanceChainHead moves the head to the just-written event.
func advanceChainHead(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, seq int64, hash []byte) error {
	if _, err := tx.Exec(ctx,
		`UPDATE audit_chain_head SET last_seq = $2, head_hash = $3, updated_at = now()
		 WHERE organization_id = $1`,
		orgID.String(), seq, hash); err != nil {
		return fmt.Errorf("postgres: avanço do cabeçalho de cadeia falhou: %w", err)
	}
	return nil
}

// insertAuditEvent writes the sealed event's columns. The delegation chain
// (actor.act) is stored as jsonb; the verifier reconstructs the event from
// these columns and recomputes the canonical form.
func insertAuditEvent(ctx context.Context, tx pgx.Tx, s domain.SealedEvent) error {
	e := s.Event
	var actJSON []byte
	if e.Actor.Act != nil {
		var err error
		if actJSON, err = json.Marshal(e.Actor.Act); err != nil {
			return fmt.Errorf("postgres: serialização do ator delegado falhou: %w", err)
		}
	}
	amr := e.Context.AuthMethods
	if amr == nil {
		amr = []string{}
	}
	const q = `
		INSERT INTO audit_event (
			organization_id, seq, occurred_at, event_id, schema_version, action, outcome,
			actor_subject, actor_membership_id, actor_session_id, actor_act,
			target_type, target_id, target_label, reason,
			context_ip, context_user_agent, context_acr, context_amr, context_trace_id, context_pcid,
			prev_hash, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`
	_, err := tx.Exec(ctx, q,
		e.OrganizationID.String(), s.Seq, e.OccurredAt, e.EventID.String(), e.SchemaVersion,
		string(e.Action), e.SerializedOutcome(),
		e.Actor.IdentitySubject, uuidTextOrNil(e.Actor.MembershipID), uuidTextOrNil(e.Actor.SessionID), actJSON,
		e.Target.Type, e.Target.ID, e.Target.Label, e.Reason,
		e.Context.IP, e.Context.UserAgent, e.Context.AuthContextClass, amr, e.Context.TraceID, e.Context.PrivilegedCorrelationID,
		s.PrevHash, s.Hash)
	if err != nil {
		return fmt.Errorf("postgres: inserção de audit_event falhou: %w", err)
	}
	return nil
}
