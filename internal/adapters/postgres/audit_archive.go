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
	"regexp"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// safeIdent guards the partition table name that is interpolated into DDL (which
// cannot be parameterized). Only simple identifiers are allowed.
var safeIdent = regexp.MustCompile(`^audit_event_[a-z0-9_]+$`)

// ErrPartitionUnsealed is returned when an archive is attempted on a time range
// that still has events not covered by a seal. Archiving unsealed events would
// remove the ability to seal them (the seal covers a contiguous prefix), so it
// is refused — retention is by archiving SEALED partitions only (RFC-0003 §5.4).
var ErrPartitionUnsealed = errors.New("audit: partição tem eventos não selados — arquivamento recusado")

// EnsureTimePartition creates a time-range partition of audit_event for
// [from,to) if it does not exist. Time partitions let retention move whole
// sealed periods to cold storage without deleting individual events (RFC-0003
// §5.4). name must match audit_event_<suffix>.
func EnsureTimePartition(ctx context.Context, db Beginner, name string, from, to time.Time) error {
	if !safeIdent.MatchString(name) {
		return fmt.Errorf("audit: nome de partição inválido %q", name)
	}
	// Bounds are literals (DDL cannot bind them); from/to are controlled inputs,
	// formatted as RFC3339 UTC.
	ddl := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_event FOR VALUES FROM ('%s') TO ('%s')`,
		name, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	return WithTx(ctx, db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, ddl)
		if err != nil {
			return fmt.Errorf("audit: criação da partição %s falhou: %w", name, err)
		}
		return nil
	})
}

// PartitionArchiver archives and restores sealed time-partitions of the audit
// trail (RFC-0003 §5.4, T-018). Archiving DETACHes the partition — the rows are
// PRESERVED in the now-standalone table (nothing is deleted), ready to be moved
// to cold storage — after confirming the range is fully sealed. Restoration
// re-ATTACHes it. Both are audited administrative operations (fail-closed via
// the shared audit emitter).
type PartitionArchiver struct {
	db    Beginner
	audit AuditEmitter
}

// NewPartitionArchiver wires the connection with an optional audit emitter.
func NewPartitionArchiver(db Beginner, audit AuditEmitter) *PartitionArchiver {
	return &PartitionArchiver{db: db, audit: audit}
}

// Archive detaches the partition covering [from,to). It refuses if any event in
// that range is not yet sealed (ErrPartitionUnsealed). The detach preserves all
// rows in the standalone table `name`. The operation is audited per affected
// organization before the detach.
func (a *PartitionArchiver) Archive(ctx context.Context, name string, from, to time.Time) error {
	if !safeIdent.MatchString(name) {
		return fmt.Errorf("audit: nome de partição inválido %q", name)
	}
	if err := a.assertRangeSealed(ctx, from, to); err != nil {
		return err
	}
	if err := a.auditPeriod(ctx, from, to, "arquivamento de partição selada"); err != nil {
		return err
	}
	return WithTx(ctx, a.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE audit_event DETACH PARTITION %s`, name)); err != nil {
			return fmt.Errorf("audit: detach da partição %s falhou: %w", name, err)
		}
		return nil
	})
}

// Restore re-attaches an archived partition table `name` for the range
// [from,to). The restoration is audited (RFC-0003 §5.4: restauração é auditada).
func (a *PartitionArchiver) Restore(ctx context.Context, name string, from, to time.Time) error {
	if !safeIdent.MatchString(name) {
		return fmt.Errorf("audit: nome de partição inválido %q", name)
	}
	if err := WithTx(ctx, a.db, func(tx pgx.Tx) error {
		ddl := fmt.Sprintf(`ALTER TABLE audit_event ATTACH PARTITION %s FOR VALUES FROM ('%s') TO ('%s')`,
			name, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
		if _, err := tx.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("audit: attach da partição %s falhou: %w", name, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return a.auditPeriod(ctx, from, to, "restauração de partição arquivada")
}

// assertRangeSealed fails if any event whose occurred_at is in [from,to) has a
// seq beyond its organization's highest sealed seq_end.
func (a *PartitionArchiver) assertRangeSealed(ctx context.Context, from, to time.Time) error {
	var unsealed int
	err := WithTx(ctx, a.db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT count(*)
			FROM audit_event e
			LEFT JOIN (SELECT organization_id, max(seq_end) m FROM audit_seal GROUP BY organization_id) s
			  ON s.organization_id = e.organization_id
			WHERE e.occurred_at >= $1 AND e.occurred_at < $2
			  AND e.seq > COALESCE(s.m, 0)`, from.UTC(), to.UTC()).Scan(&unsealed)
	})
	if err != nil {
		return fmt.Errorf("audit: verificação de selagem da partição falhou: %w", err)
	}
	if unsealed > 0 {
		return fmt.Errorf("%w: %d evento(s) não selado(s) no período", ErrPartitionUnsealed, unsealed)
	}
	return nil
}

// auditPeriod records one admin-mutation audit event per organization that has
// events in [from,to), atomically. Skipped when no emitter is wired.
func (a *PartitionArchiver) auditPeriod(ctx context.Context, from, to time.Time, reason string) error {
	if a.audit == nil {
		return nil
	}
	var orgs []uuid.UUID
	if err := WithTx(ctx, a.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT DISTINCT organization_id::text FROM audit_event
			 WHERE occurred_at >= $1 AND occurred_at < $2 ORDER BY 1`, from.UTC(), to.UTC())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return err
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return err
			}
			orgs = append(orgs, id)
		}
		return rows.Err()
	}); err != nil {
		return fmt.Errorf("audit: leitura das organizações do período falhou: %w", err)
	}
	for _, org := range orgs {
		if err := WithTx(ctx, a.db, func(tx pgx.Tx) error {
			return emitAudit(ctx, tx, a.audit, org, domain.ActionAdminMutation,
				domain.AuditTarget{Type: "audit_partition", ID: from.UTC().Format(time.RFC3339), Label: reason},
				reason)
		}); err != nil {
			return err
		}
	}
	return nil
}
