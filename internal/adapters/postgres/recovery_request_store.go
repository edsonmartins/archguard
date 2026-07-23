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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrRecoveryNotFound is returned when a recovery request does not exist within
// the store's tenant scope.
var ErrRecoveryNotFound = errors.New("postgres: solicitação de recuperação não encontrada")

// ErrCrossTenantRecovery is returned when a tenant-scoped recovery store is asked
// to persist a request of another organization.
var ErrCrossTenantRecovery = errors.New("postgres: recuperação de outra organização recusada")

// RecoveryRequestStore is the tenant-scoped store for the peer-approved recovery
// workflow (T-013). Built on a TenantTx, it carries the explicit organization_id
// predicate (Barreira 1) and the SET LOCAL tenant setting the RLS policy reads
// (Barreira 2). The peers who approve are members of this tenant.
type RecoveryRequestStore struct {
	ttx *TenantTx
}

// NewRecoveryRequestStore builds the store on an open tenant transaction.
func NewRecoveryRequestStore(ttx *TenantTx) *RecoveryRequestStore {
	return &RecoveryRequestStore{ttx: ttx}
}

// Create persists a freshly opened (pending) request. It refuses a request of
// another organization than the store's scope.
func (s *RecoveryRequestStore) Create(ctx context.Context, r domain.RecoveryRequest) error {
	if r.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantRecovery, r.OrganizationID, s.ttx.scope.OrganizationID())
	}
	const q = `
		INSERT INTO recovery_request (id, target_identity_id, organization_id, requested_by, justification, status, required_approvals)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.ttx.tx.Exec(ctx, q,
		r.ID.String(), r.TargetIdentityID.String(), r.OrganizationID.String(),
		r.RequestedBy.String(), r.Justification, string(r.Status), r.RequiredApprovals)
	if err != nil {
		return fmt.Errorf("postgres: criação de recuperação falhou: %w", err)
	}
	return nil
}

// Get loads a request and its approvals within the tenant scope.
func (s *RecoveryRequestStore) Get(ctx context.Context, id uuid.UUID) (domain.RecoveryRequest, error) {
	const q = `
		SELECT id::text, target_identity_id::text, organization_id::text, requested_by::text,
		       justification, status, required_approvals
		FROM recovery_request
		WHERE id = $1 AND organization_id = $2`
	var r domain.RecoveryRequest
	var idText, targetText, orgText, reqText, status string
	err := s.ttx.tx.QueryRow(ctx, q, id.String(), s.ttx.scope.OrganizationID().String()).
		Scan(&idText, &targetText, &orgText, &reqText, &r.Justification, &status, &r.RequiredApprovals)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecoveryRequest{}, ErrRecoveryNotFound
	}
	if err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: leitura de recuperação falhou: %w", err)
	}
	if r.ID, err = uuid.Parse(idText); err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: id de recuperação inválido: %w", err)
	}
	if r.TargetIdentityID, err = uuid.Parse(targetText); err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: target inválido: %w", err)
	}
	if r.OrganizationID, err = uuid.Parse(orgText); err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: organização inválida: %w", err)
	}
	if r.RequestedBy, err = uuid.Parse(reqText); err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: requested_by inválido: %w", err)
	}
	r.Status = domain.RecoveryStatus(status)

	rows, err := s.ttx.tx.Query(ctx,
		`SELECT approver_identity_id::text FROM recovery_approval
		 WHERE recovery_request_id = $1 AND organization_id = $2
		 ORDER BY created_at`,
		id.String(), s.ttx.scope.OrganizationID().String())
	if err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: leitura de aprovações falhou: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var approverText string
		if err := rows.Scan(&approverText); err != nil {
			return domain.RecoveryRequest{}, fmt.Errorf("postgres: leitura de aprovação falhou: %w", err)
		}
		approver, err := uuid.Parse(approverText)
		if err != nil {
			return domain.RecoveryRequest{}, fmt.Errorf("postgres: aprovador inválido: %w", err)
		}
		r.Approvals = append(r.Approvals, domain.RecoveryApproval{ApproverIdentityID: approver})
	}
	if err := rows.Err(); err != nil {
		return domain.RecoveryRequest{}, fmt.Errorf("postgres: iteração de aprovações falhou: %w", err)
	}
	return r, nil
}

// SaveDecision persists the outcome of a domain transition (Approve/Reject/
// MarkConsumed already ran on r): it upserts every approval (the composite PK
// enforces distinct approvers) and writes the new status, within the caller's
// transaction. It refuses a request of another organization.
func (s *RecoveryRequestStore) SaveDecision(ctx context.Context, r domain.RecoveryRequest) error {
	if r.OrganizationID != s.ttx.scope.OrganizationID() {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantRecovery, r.OrganizationID, s.ttx.scope.OrganizationID())
	}
	for _, a := range r.Approvals {
		if _, err := s.ttx.tx.Exec(ctx,
			`INSERT INTO recovery_approval (recovery_request_id, approver_identity_id, organization_id)
			 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			r.ID.String(), a.ApproverIdentityID.String(), r.OrganizationID.String()); err != nil {
			return fmt.Errorf("postgres: gravação de aprovação falhou: %w", err)
		}
	}
	tag, err := s.ttx.tx.Exec(ctx,
		`UPDATE recovery_request SET status = $3, updated_at = now()
		 WHERE id = $1 AND organization_id = $2`,
		r.ID.String(), r.OrganizationID.String(), string(r.Status))
	if err != nil {
		return fmt.Errorf("postgres: atualização de recuperação falhou: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRecoveryNotFound
	}
	return nil
}
