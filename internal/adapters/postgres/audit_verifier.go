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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuditVerifier recomputes an organization's chain from the stored columns and
// checks the seal signatures (RFC-0003 §6). It reconstructs each event from its
// columns (NOT from a stored canonical blob — so tampering with any queryable
// column changes the recomputed canonical and breaks the hash), walks the chain
// with domain.VerifyChain, then verifies each seal's signature and head against
// the events. It reports the FIRST divergence and its kind.
type AuditVerifier struct {
	db       Beginner
	verifier domain.SealVerifier
}

// NewAuditVerifier wires the connection with the seal signature verifier. The
// verifier may be NIL: then the chain and the seal STRUCTURE (contiguity, head
// match) are still verified — catching alteration, removal and reorder — but the
// seal SIGNATURES are not (that needs the custodied public keys, e.g. the vault
// in production). A dev CLI without vault access runs in this mode.
func NewAuditVerifier(db Beginner, verifier domain.SealVerifier) *AuditVerifier {
	return &AuditVerifier{db: db, verifier: verifier}
}

// VerifyOrganization verifies the whole chain and all seals of one organization.
func (v *AuditVerifier) VerifyOrganization(ctx context.Context, orgID uuid.UUID) (domain.VerifyReport, error) {
	genesis, hasHead, err := v.genesis(ctx, orgID)
	if err != nil {
		return domain.VerifyReport{}, err
	}
	if !hasHead {
		return domain.VerifyReport{OK: true}, nil // no chain yet
	}

	events, err := v.readEvents(ctx, orgID)
	if err != nil {
		return domain.VerifyReport{}, err
	}
	rep := domain.VerifyChain(genesis, events)
	if !rep.OK {
		return rep, nil
	}

	// Index event hashes by seq for the seal head check.
	hashBySeq := make(map[int64][]byte, len(events))
	for _, e := range events {
		hashBySeq[e.Seq] = e.Hash
	}
	sealRep, err := v.verifySeals(ctx, orgID, hashBySeq)
	if err != nil {
		return domain.VerifyReport{}, err
	}
	if !sealRep.OK {
		sealRep.EventsChecked = rep.EventsChecked
		return sealRep, nil
	}
	rep.SealsChecked = sealRep.SealsChecked
	rep.SealSignaturesChecked = v.verifier != nil
	return rep, nil
}

// genesis returns the org's genesis hash and whether a chain head exists.
func (v *AuditVerifier) genesis(ctx context.Context, orgID uuid.UUID) ([]byte, bool, error) {
	return readGenesisHash(ctx, v.db, orgID)
}

// readGenesisHash returns the org's genesis hash and whether a chain head
// exists — shared by the verifier (T-013) and the exporter (T-016).
func readGenesisHash(ctx context.Context, db Beginner, orgID uuid.UUID) ([]byte, bool, error) {
	var nonce []byte
	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT genesis_nonce FROM audit_chain_head WHERE organization_id = $1`, orgID.String()).Scan(&nonce)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("audit: leitura da gênese falhou: %w", err)
	}
	g, err := domain.GenesisHash(orgID, nonce)
	if err != nil {
		return nil, false, err
	}
	return g, true, nil
}

// readEvents reconstructs the org's events (ordered by seq) into SealedEvents.
func (v *AuditVerifier) readEvents(ctx context.Context, orgID uuid.UUID) ([]domain.SealedEvent, error) {
	return readSealedEvents(ctx, v.db, orgID)
}

// readSealedEvents reconstructs an organization's events (ordered by seq) from
// their columns — shared by the verifier (T-013) and the exporter (T-016).
func readSealedEvents(ctx context.Context, db Beginner, orgID uuid.UUID) ([]domain.SealedEvent, error) {
	var out []domain.SealedEvent
	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT seq, occurred_at, event_id::text, schema_version, action, outcome,
			       actor_subject, actor_membership_id::text, actor_session_id::text, actor_act,
			       target_type, target_id, target_label, reason,
			       context_ip, context_user_agent, context_acr, context_amr, context_trace_id, context_pcid,
			       prev_hash, hash
			FROM audit_event WHERE organization_id = $1 ORDER BY seq`, orgID.String())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			se, err := scanSealedEvent(rows, orgID)
			if err != nil {
				return err
			}
			out = append(out, se)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("audit: leitura de eventos para verificação falhou: %w", err)
	}
	return out, nil
}

// scanSealedEvent reconstructs one SealedEvent from the audit_event columns —
// the inverse of insertAuditEvent.
func scanSealedEvent(row pgx.Row, orgID uuid.UUID) (domain.SealedEvent, error) {
	var (
		se                   domain.SealedEvent
		e                    domain.AuditEvent
		eventIDText, outcome string
		memText, sesText     *string
		actJSON              []byte
	)
	if err := row.Scan(&se.Seq, &e.OccurredAt, &eventIDText, &e.SchemaVersion, &e.Action, &outcome,
		&e.Actor.IdentitySubject, &memText, &sesText, &actJSON,
		&e.Target.Type, &e.Target.ID, &e.Target.Label, &e.Reason,
		&e.Context.IP, &e.Context.UserAgent, &e.Context.AuthContextClass, &e.Context.AuthMethods,
		&e.Context.TraceID, &e.Context.PrivilegedCorrelationID,
		&se.PrevHash, &se.Hash); err != nil {
		return domain.SealedEvent{}, err
	}
	var err error
	if e.EventID, err = uuid.Parse(eventIDText); err != nil {
		return domain.SealedEvent{}, fmt.Errorf("audit: event_id inválido %q: %w", eventIDText, err)
	}
	if e.Outcome, err = domain.ParseOutcome(outcome); err != nil {
		return domain.SealedEvent{}, err
	}
	if e.Actor.MembershipID, err = parseOptionalUUID("actor_membership_id", memText); err != nil {
		return domain.SealedEvent{}, err
	}
	if e.Actor.SessionID, err = parseOptionalUUID("actor_session_id", sesText); err != nil {
		return domain.SealedEvent{}, err
	}
	if len(actJSON) > 0 {
		var act domain.AuditActor
		if err := json.Unmarshal(actJSON, &act); err != nil {
			return domain.SealedEvent{}, fmt.Errorf("audit: actor_act inválido: %w", err)
		}
		e.Actor.Act = &act
	}
	e.OrganizationID = orgID
	se.Event = e
	return se, nil
}

// verifySeals checks every seal: contiguity of the sealed ranges, the head hash
// matching the event at seq_end, and the Ed25519 signature. It reports the first
// invalid seal.
func (v *AuditVerifier) verifySeals(ctx context.Context, orgID uuid.UUID, hashBySeq map[int64][]byte) (domain.VerifyReport, error) {
	type sealRow struct {
		seqStart, seqEnd int64
		headHash, sig    []byte
		keyID            string
		sealedAtMicros   int64
	}
	var seals []sealRow
	err := WithTx(ctx, v.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT seq_start, seq_end, head_hash, sealed_at, key_id, signature
			FROM audit_seal WHERE organization_id = $1 ORDER BY seq_start`, orgID.String())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s sealRow
			var sealedAt time.Time
			if err := rows.Scan(&s.seqStart, &s.seqEnd, &s.headHash, &sealedAt, &s.keyID, &s.sig); err != nil {
				return err
			}
			s.sealedAtMicros = sealedAt.UTC().UnixMicro()
			seals = append(seals, s)
		}
		return rows.Err()
	})
	if err != nil {
		return domain.VerifyReport{}, fmt.Errorf("audit: leitura de selos para verificação falhou: %w", err)
	}

	var prevEnd int64
	checked := 0
	for _, s := range seals {
		// Contiguidade: o intervalo do selo segue imediatamente o anterior.
		if s.seqStart != prevEnd+1 {
			return failSeal(s.seqEnd, "intervalo de selos não contíguo: esperado início %d, veio %d", prevEnd+1, s.seqStart), nil
		}
		// O head do selo deve ser o hash do evento em seq_end.
		wantHead, ok := hashBySeq[s.seqEnd]
		if !ok || !bytesEqualBytes(wantHead, s.headHash) {
			return failSeal(s.seqEnd, "head_hash do selo não corresponde ao evento em seq %d", s.seqEnd), nil
		}
		// Assinatura — só quando há verificador (chaves custodiadas disponíveis).
		if v.verifier != nil {
			content, err := domain.SealContent(orgID, s.seqStart, s.seqEnd, s.headHash, s.sealedAtMicros)
			if err != nil {
				return failSeal(s.seqEnd, "conteúdo do selo inválido: %v", err), nil
			}
			valid, err := v.verifier.Verify(ctx, content, s.sig, s.keyID)
			if err != nil {
				return failSeal(s.seqEnd, "verificação da assinatura falhou: %v", err), nil
			}
			if !valid {
				return failSeal(s.seqEnd, "assinatura do selo inválida"), nil
			}
		}
		prevEnd = s.seqEnd
		checked++
	}
	return domain.VerifyReport{OK: true, SealsChecked: checked}, nil
}

func failSeal(seqEnd int64, format string, args ...any) domain.VerifyReport {
	return domain.VerifyReport{OK: false, FirstDivergence: seqEnd, Kind: domain.DivergenceSealInvalid, Detail: fmt.Sprintf(format, args...)}
}

func bytesEqualBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
