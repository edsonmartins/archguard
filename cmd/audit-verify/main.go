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

// Command audit-verify runs the immutable-audit-trail verifier (RFC-0003 §6,
// pacote 003 T-014) from the command line: it recomputes each organization's
// chain and checks its seals, printing a report per organization and exiting
// NON-ZERO on any divergence — so a cron/CI job alerts on the exit status
// alone. Without vault access it verifies the chain and seal structure (which
// catches alteration, removal and reorder); seal signature verification needs
// the custodied keys and runs when the vault verifier is wired.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Exit codes: 0 intact, 1 divergence detected (the alerting signal), 2 error.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point: it parses args, verifies, and returns the
// process exit code without calling os.Exit.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit-verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dsn := fs.String("dsn", os.Getenv("ARCHGUARD_DSN"), "PostgreSQL DSN (ou env ARCHGUARD_DSN)")
	orgFlag := fs.String("org", "", "organization_id (uuid) a verificar; vazio = todas")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dsn == "" {
		fmt.Fprintln(stderr, "erro: informe -dsn ou a variável ARCHGUARD_DSN")
		return 2
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, *dsn)
	if err != nil {
		fmt.Fprintf(stderr, "erro: conexão falhou: %v\n", err)
		return 2
	}
	defer pool.Close()

	orgs, err := targetOrgs(ctx, pool, *orgFlag)
	if err != nil {
		fmt.Fprintf(stderr, "erro: %v\n", err)
		return 2
	}

	// Nil verifier: chain + seal structure (no vault here). The production
	// deployment wires the vault-backed SealVerifier for signature checks.
	verifier := postgres.NewAuditVerifier(pool, nil)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	diverged := false
	for _, org := range orgs {
		rep, err := verifier.VerifyOrganization(ctx, org)
		if err != nil {
			// Fail-closed: a verification that cannot run is a failure, not "ok".
			fmt.Fprintf(stderr, "erro: verificação de %s falhou: %v\n", org, err)
			return 2
		}
		_ = enc.Encode(map[string]any{
			"organization_id":         org.String(),
			"ok":                      rep.OK,
			"events_checked":          rep.EventsChecked,
			"seals_checked":           rep.SealsChecked,
			"seal_signatures_checked": rep.SealSignaturesChecked,
			"first_divergence_seq":    rep.FirstDivergence,
			"divergence_kind":         string(rep.Kind),
			"detail":                  rep.Detail,
		})
		if !rep.OK {
			diverged = true
		}
	}

	if diverged {
		return 1 // divergence detected — the alerting signal
	}
	return 0
}

// targetOrgs returns the organizations to verify: the one requested, or all
// organizations that have an audit chain.
func targetOrgs(ctx context.Context, pool *pgxpool.Pool, orgFlag string) ([]uuid.UUID, error) {
	if orgFlag != "" {
		id, err := uuid.Parse(orgFlag)
		if err != nil {
			return nil, fmt.Errorf("organization_id inválido: %w", err)
		}
		return []uuid.UUID{id}, nil
	}
	rows, err := pool.Query(ctx, `SELECT organization_id::text FROM audit_chain_head ORDER BY organization_id`)
	if err != nil {
		return nil, fmt.Errorf("listagem de organizações falhou: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
