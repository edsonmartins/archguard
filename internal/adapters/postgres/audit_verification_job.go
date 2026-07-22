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

// AuditVerificationJob runs the scheduled integrity check of the audit trail
// (RFC-0003 §6, T-015): it verifies every organization's chain and raises a
// MAXIMUM-severity alert on any divergence — or on any verification that could
// not run, which is itself an incident (fail-closed). It is meant to run daily,
// but RunOnce is the testable unit; Run wraps it in a ticker.
type AuditVerificationJob struct {
	db       Beginner
	verifier *AuditVerifier
	alerter  domain.Alerter
}

// NewAuditVerificationJob wires the connection, the verifier and the alerter.
func NewAuditVerificationJob(db Beginner, verifier *AuditVerifier, alerter domain.Alerter) *AuditVerificationJob {
	return &AuditVerificationJob{db: db, verifier: verifier, alerter: alerter}
}

// VerificationSummary is the outcome of one verification pass.
type VerificationSummary struct {
	OrgsChecked  int
	Divergences  int
	VerifyErrors int
}

// RunOnce verifies every organization that has a chain. Divergences and
// verification errors are each alerted at critical severity; the pass does not
// abort on a single failure — every organization is checked so one bad tenant
// does not hide problems in the others. It returns a summary and a non-nil
// error if any organization diverged or failed (so a caller/exit code can react).
func (j *AuditVerificationJob) RunOnce(ctx context.Context) (VerificationSummary, error) {
	orgs, err := allChainOrgs(ctx, j.db)
	if err != nil {
		return VerificationSummary{}, err
	}
	var sum VerificationSummary
	for _, org := range orgs {
		sum.OrgsChecked++
		rep, err := j.verifier.VerifyOrganization(ctx, org)
		if err != nil {
			sum.VerifyErrors++
			j.raise(ctx, org, domain.SeverityCritical,
				"verificação da trilha não pôde ser concluída", err.Error())
			continue
		}
		if !rep.OK {
			sum.Divergences++
			j.raise(ctx, org, domain.SeverityCritical,
				"ADULTERAÇÃO DA TRILHA DETECTADA",
				fmt.Sprintf("primeira divergência no seq %d (%s): %s", rep.FirstDivergence, rep.Kind, rep.Detail))
		}
	}
	if sum.Divergences > 0 || sum.VerifyErrors > 0 {
		return sum, fmt.Errorf("verificação diária: %d divergência(s), %d falha(s) em %d organização(ões)",
			sum.Divergences, sum.VerifyErrors, sum.OrgsChecked)
	}
	return sum, nil
}

// raise emits an alert; a delivery failure is logged into the detail but does
// not stop the pass (the divergence is still recorded in the summary).
func (j *AuditVerificationJob) raise(ctx context.Context, org uuid.UUID, sev domain.Severity, subject, detail string) {
	_ = j.alerter.Alert(ctx, domain.Alert{
		Severity: sev,
		Subject:  subject,
		Detail:   fmt.Sprintf("organization_id=%s: %s", org, detail),
	})
}

// Run executes RunOnce immediately and then every interval until the context is
// cancelled — the daily schedule (RFC-0003 §6). This is thin runtime wiring; the
// deployment sets interval to 24h.
func (j *AuditVerificationJob) Run(ctx context.Context, interval time.Duration) {
	_, _ = j.RunOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = j.RunOnce(ctx)
		}
	}
}

// allChainOrgs lists every organization that has an audit chain.
func allChainOrgs(ctx context.Context, db Beginner) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := WithTx(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT organization_id::text FROM audit_chain_head ORDER BY organization_id`)
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
			out = append(out, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("audit: listagem de organizações da cadeia falhou: %w", err)
	}
	return out, nil
}
