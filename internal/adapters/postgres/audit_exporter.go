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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SealExporter anchors sealed ranges to an external WORM destination (RFC-0003
// §4, T-012) and records the anchor locally so it does not re-export. The WORM
// write is a remote call, so it happens OUTSIDE any database transaction
// (RFC-0004 §4); the local bookkeeping row is written after.
type SealExporter struct {
	db     Beginner
	anchor domain.SealAnchor
}

// NewSealExporter wires the connection with the external anchor.
func NewSealExporter(db Beginner, anchor domain.SealAnchor) *SealExporter {
	return &SealExporter{db: db, anchor: anchor}
}

// pendingSeal is a seal not yet anchored to a destination.
type pendingSeal struct {
	sealID string
	seal   domain.Seal
}

// ExportPending anchors every seal not yet anchored to destination and returns
// how many were newly anchored. Idempotent: a seal already recorded for the
// destination is skipped, and the content-addressed WORM write tolerates a
// re-anchor of identical bytes. On a WORM overwrite conflict (a ref holding
// different bytes) the export stops with the error — that is a tamper signal,
// not a routine condition.
func (e *SealExporter) ExportPending(ctx context.Context, destination string) (int, error) {
	pending, err := e.readPending(ctx, destination)
	if err != nil {
		return 0, err
	}

	exported := 0
	for _, p := range pending {
		// Remote WORM write outside any DB transaction (RFC-0004 §4).
		ref, err := e.anchor.Anchor(ctx, p.seal)
		if err != nil {
			return exported, fmt.Errorf("audit: ancoragem WORM do selo %s falhou: %w", p.sealID, err)
		}
		id, err := uuid.NewV7()
		if err != nil {
			return exported, fmt.Errorf("audit: id de âncora falhou: %w", err)
		}
		if err := WithTx(ctx, e.db, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`INSERT INTO audit_seal_anchor (id, seal_id, destination, ref)
				 VALUES ($1, $2, $3, $4) ON CONFLICT (seal_id, destination) DO NOTHING`,
				id.String(), p.sealID, destination, ref)
			return err
		}); err != nil {
			return exported, fmt.Errorf("audit: registro de âncora do selo %s falhou: %w", p.sealID, err)
		}
		exported++
	}
	return exported, nil
}

// readPending returns the seals not yet anchored to destination.
func (e *SealExporter) readPending(ctx context.Context, destination string) ([]pendingSeal, error) {
	var out []pendingSeal
	err := WithTx(ctx, e.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id::text, s.organization_id::text, s.seq_start, s.seq_end,
			       s.head_hash, s.sealed_at, s.key_id, s.signature
			FROM audit_seal s
			LEFT JOIN audit_seal_anchor a
			  ON a.seal_id = s.id AND a.destination = $1
			WHERE a.id IS NULL
			ORDER BY s.organization_id, s.seq_start`, destination)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				p        pendingSeal
				orgText  string
				sealedAt time.Time
			)
			if err := rows.Scan(&p.sealID, &orgText, &p.seal.SeqStart, &p.seal.SeqEnd,
				&p.seal.HeadHash, &sealedAt, &p.seal.KeyID, &p.seal.Signature); err != nil {
				return err
			}
			org, err := uuid.Parse(orgText)
			if err != nil {
				return err
			}
			p.seal.OrganizationID = org
			p.seal.SealedAt = sealedAt.UTC().UnixMicro()
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("audit: leitura de selos pendentes falhou: %w", err)
	}
	return out, nil
}
