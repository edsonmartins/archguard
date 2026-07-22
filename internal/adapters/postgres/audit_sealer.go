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
	"errors"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SealConfig is the interval/volume policy of periodic sealing (RFC-0003 §4:
// default 1h or 10.000 events). An organization is due for a seal when it has
// unsealed events AND either the volume threshold is reached OR the oldest
// unsealed event / last seal is older than the interval.
type SealConfig struct {
	Volume   int64
	Interval time.Duration
}

// DefaultSealConfig is the RFC-0003 §4 default policy.
var DefaultSealConfig = SealConfig{Volume: 10_000, Interval: time.Hour}

// AuditSealer seals the head of each organization's chain (RFC-0003 §4). The
// signature comes from the domain Sealer (the vault in production, ADR-0012);
// the signing call is made OUTSIDE any database transaction (RFC-0004 §4). The
// seal row is append-only (migration 0020) — a sealed range cannot be altered
// or removed undetectably.
type AuditSealer struct {
	db     Beginner
	sealer domain.Sealer
	clock  Clock
}

// NewAuditSealer wires the connection, the signer and a clock (nil → time.Now).
func NewAuditSealer(db Beginner, sealer domain.Sealer, clock Clock) *AuditSealer {
	if clock == nil {
		clock = time.Now
	}
	return &AuditSealer{db: db, sealer: sealer, clock: clock}
}

// SealOrganization seals the pending range of one organization's chain: from the
// event after the last seal through the current head. It returns the created
// seal and true, or (zero, false) when there is nothing new to seal. The head
// is read, the content SIGNED outside any transaction, then the seal inserted;
// a concurrent duplicate is caught by the UNIQUE(organization_id, seq_end).
func (s *AuditSealer) SealOrganization(ctx context.Context, orgID uuid.UUID) (domain.Seal, bool, error) {
	lastSeq, headHash, hasHead, err := s.readHead(ctx, orgID)
	if err != nil {
		return domain.Seal{}, false, err
	}
	lastEnd, err := s.lastSealedSeq(ctx, orgID)
	if err != nil {
		return domain.Seal{}, false, err
	}
	if !hasHead || lastSeq <= lastEnd {
		return domain.Seal{}, false, nil // nothing new to seal
	}

	seqStart := lastEnd + 1
	sealedAt := s.clock().UTC()
	content, err := domain.SealContent(orgID, seqStart, lastSeq, headHash, sealedAt.UnixMicro())
	if err != nil {
		return domain.Seal{}, false, err
	}
	// Sign OUTSIDE any transaction (RFC-0004 §4: no remote call inside a DB tx).
	signature, keyID, err := s.sealer.Sign(ctx, content)
	if err != nil {
		return domain.Seal{}, false, fmt.Errorf("audit: assinatura do selo falhou: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Seal{}, false, fmt.Errorf("audit: id do selo falhou: %w", err)
	}
	_, err = s.exec(ctx,
		`INSERT INTO audit_seal (id, organization_id, seq_start, seq_end, head_hash, sealed_at, key_id, signature)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (organization_id, seq_end) DO NOTHING`,
		id.String(), orgID.String(), seqStart, lastSeq, headHash, sealedAt, keyID, signature)
	if err != nil {
		return domain.Seal{}, false, fmt.Errorf("audit: persistência do selo falhou: %w", err)
	}
	return domain.Seal{
		OrganizationID: orgID, SeqStart: seqStart, SeqEnd: lastSeq, HeadHash: headHash,
		SealedAt: sealedAt.UnixMicro(), KeyID: keyID, Signature: signature,
	}, true, nil
}

// SealDue seals every organization due under cfg and returns how many were
// sealed. Due = has unsealed events AND (volume reached OR baseline older than
// the interval), where baseline is the last seal time, or the oldest unsealed
// event when the organization was never sealed.
func (s *AuditSealer) SealDue(ctx context.Context, cfg SealConfig) (int, error) {
	cutoff := s.clock().UTC().Add(-cfg.Interval)

	type candidate struct {
		org         uuid.UUID
		unsealed    int64
		baseline    time.Time
		hasBaseline bool
	}
	var due []candidate
	if err := WithTx(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT ch.organization_id::text,
			       ch.last_seq - COALESCE(s.max_end, 0) AS unsealed,
			       COALESCE(s.last_sealed_at,
			                (SELECT MIN(e.occurred_at) FROM audit_event e
			                 WHERE e.organization_id = ch.organization_id AND e.seq > COALESCE(s.max_end, 0))) AS baseline
			FROM audit_chain_head ch
			LEFT JOIN (
			    SELECT organization_id, MAX(seq_end) AS max_end, MAX(sealed_at) AS last_sealed_at
			    FROM audit_seal GROUP BY organization_id
			) s ON s.organization_id = ch.organization_id
			WHERE ch.last_seq > COALESCE(s.max_end, 0)`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c candidate
			var orgText string
			var baseline *time.Time
			if err := rows.Scan(&orgText, &c.unsealed, &baseline); err != nil {
				return err
			}
			c.org, err = uuid.Parse(orgText)
			if err != nil {
				return err
			}
			if baseline != nil {
				c.baseline, c.hasBaseline = *baseline, true
			}
			if c.unsealed >= cfg.Volume || (c.hasBaseline && !c.baseline.After(cutoff)) {
				due = append(due, c)
			}
		}
		return rows.Err()
	}); err != nil {
		return 0, fmt.Errorf("audit: seleção de organizações a selar falhou: %w", err)
	}

	sealed := 0
	for _, c := range due {
		if _, ok, err := s.SealOrganization(ctx, c.org); err != nil {
			return sealed, err
		} else if ok {
			sealed++
		}
	}
	return sealed, nil
}

// readHead returns the org's last seq and head hash, and whether a head exists.
func (s *AuditSealer) readHead(ctx context.Context, orgID uuid.UUID) (int64, []byte, bool, error) {
	var lastSeq int64
	var headHash []byte
	err := WithTx(ctx, s.db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT last_seq, head_hash FROM audit_chain_head WHERE organization_id = $1`, orgID.String()).
			Scan(&lastSeq, &headHash)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, fmt.Errorf("audit: leitura do cabeçalho para selagem falhou: %w", err)
	}
	return lastSeq, headHash, true, nil
}

// lastSealedSeq returns the highest sealed seq_end for the org, or 0 if none.
func (s *AuditSealer) lastSealedSeq(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var lastEnd int64
	err := WithTx(ctx, s.db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(seq_end), 0) FROM audit_seal WHERE organization_id = $1`, orgID.String()).
			Scan(&lastEnd)
	})
	if err != nil {
		return 0, fmt.Errorf("audit: leitura do último selo falhou: %w", err)
	}
	return lastEnd, nil
}

// exec runs a write on its own transaction.
func (s *AuditSealer) exec(ctx context.Context, sql string, args ...any) (int64, error) {
	var affected int64
	err := WithTx(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return err
		}
		affected = tag.RowsAffected()
		return nil
	})
	return affected, err
}
