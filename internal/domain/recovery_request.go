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

package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// DefaultRecoveryApprovals is the number of peer approvals a recovery needs when
// the caller does not specify one — recovery is a break-glass path, so more than
// one peer must agree (ADR-0010 / spec "Recuperação sem credencial administrativa
// universal": no single actor can reset a factor).
const DefaultRecoveryApprovals = 2

// RecoveryStatus is the lifecycle of a factor-recovery request. The machine is:
// pending → approved → consumed, or pending → rejected. approved/rejected/consumed
// are terminal for new approvals; only an approved request may be consumed (the
// reset performed), which is what makes "no reset without approval" structural.
type RecoveryStatus string

const (
	RecoveryPending  RecoveryStatus = "pending"
	RecoveryApproved RecoveryStatus = "approved"
	RecoveryRejected RecoveryStatus = "rejected"
	RecoveryConsumed RecoveryStatus = "consumed"
)

// Valid reports whether s is a defined status.
func (s RecoveryStatus) Valid() bool {
	switch s {
	case RecoveryPending, RecoveryApproved, RecoveryRejected, RecoveryConsumed:
		return true
	default:
		return false
	}
}

// Errors of the recovery workflow. Each denial is DISTINCT so the audit trail and
// the caller can tell a policy refusal (separation of duties, duplicate) from a
// state error.
var (
	ErrInvalidRecoveryRequest = errors.New("recovery: dados obrigatórios ausentes")
	// ErrRecoveryNotPending is returned when approving or rejecting a request that
	// is no longer pending (already resolved).
	ErrRecoveryNotPending = errors.New("recovery: solicitação não está mais pendente")
	// ErrApproverIsTarget enforces separation of duties: the identity being
	// recovered cannot approve its own recovery.
	ErrApproverIsTarget = errors.New("recovery: o alvo da recuperação não pode aprovar a própria recuperação")
	// ErrApproverIsRequester enforces separation of duties: whoever opened the
	// request cannot also be one of its approvers.
	ErrApproverIsRequester = errors.New("recovery: quem solicita a recuperação não pode aprová-la")
	// ErrDuplicateApproval is returned when the same peer approves twice — the
	// threshold must be met by DISTINCT peers.
	ErrDuplicateApproval = errors.New("recovery: aprovação duplicada do mesmo par")
	// ErrRecoveryNotApproved is returned when consuming (performing the reset of) a
	// request that is not approved — the reset can only follow approval.
	ErrRecoveryNotApproved = errors.New("recovery: reset exige solicitação aprovada")
)

// RecoveryApproval is one peer's approval of a recovery request.
type RecoveryApproval struct {
	ApproverIdentityID uuid.UUID
}

// RecoveryRequest is a request to recover access for an identity that lost its
// authenticator and has no recovery code (spec "Perda de dispositivo"). It
// carries a mandatory justification and requires a THRESHOLD of distinct PEER
// approvals before the factor may be reset — there is no universal administrative
// credential that resets a factor silently. Every transition is meant to be
// audited and the target notified (the caller wires the audit/notify; this type
// enforces the rules).
type RecoveryRequest struct {
	ID               uuid.UUID
	TargetIdentityID uuid.UUID
	// OrganizationID is the tenant whose peers approve — recovery happens in a
	// tenant context (the approvers are members of that organization).
	OrganizationID uuid.UUID
	// RequestedBy is who opened the request. It MAY equal the target (self-service
	// recovery), but can never be one of the approvers.
	RequestedBy       uuid.UUID
	Justification     string
	Status            RecoveryStatus
	RequiredApprovals int
	Approvals         []RecoveryApproval
}

// NewRecoveryRequest opens a pending request. justification is mandatory (the
// spec demands it); requiredApprovals defaults to DefaultRecoveryApprovals when
// zero and must be at least 1.
func NewRecoveryRequest(target, organizationID, requestedBy uuid.UUID, justification string, requiredApprovals int) (RecoveryRequest, error) {
	if target == uuid.Nil || organizationID == uuid.Nil || requestedBy == uuid.Nil {
		return RecoveryRequest{}, fmt.Errorf("%w: identidades/organização", ErrInvalidRecoveryRequest)
	}
	if justification == "" {
		return RecoveryRequest{}, fmt.Errorf("%w: justificativa obrigatória", ErrInvalidRecoveryRequest)
	}
	if requiredApprovals == 0 {
		requiredApprovals = DefaultRecoveryApprovals
	}
	if requiredApprovals < 1 {
		return RecoveryRequest{}, fmt.Errorf("%w: aprovações exigidas inválidas: %d", ErrInvalidRecoveryRequest, requiredApprovals)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return RecoveryRequest{}, fmt.Errorf("recovery: geração de UUIDv7 falhou: %w", err)
	}
	return RecoveryRequest{
		ID:                id,
		TargetIdentityID:  target,
		OrganizationID:    organizationID,
		RequestedBy:       requestedBy,
		Justification:     justification,
		Status:            RecoveryPending,
		RequiredApprovals: requiredApprovals,
	}, nil
}

// Approve records a peer's approval, enforcing separation of duties: the approver
// may be neither the target nor the requester, and may not approve twice. When
// the count of distinct approvals reaches RequiredApprovals the request becomes
// approved. Only a pending request accepts approvals.
func (r *RecoveryRequest) Approve(approver uuid.UUID) error {
	if r.Status != RecoveryPending {
		return fmt.Errorf("%w: status %s", ErrRecoveryNotPending, r.Status)
	}
	if approver == uuid.Nil {
		return fmt.Errorf("%w: aprovador nulo", ErrInvalidRecoveryRequest)
	}
	if approver == r.TargetIdentityID {
		return ErrApproverIsTarget
	}
	if approver == r.RequestedBy {
		return ErrApproverIsRequester
	}
	for _, a := range r.Approvals {
		if a.ApproverIdentityID == approver {
			return ErrDuplicateApproval
		}
	}
	r.Approvals = append(r.Approvals, RecoveryApproval{ApproverIdentityID: approver})
	if len(r.Approvals) >= r.RequiredApprovals {
		r.Status = RecoveryApproved
	}
	return nil
}

// Reject terminates a pending request without a reset.
func (r *RecoveryRequest) Reject() error {
	if r.Status != RecoveryPending {
		return fmt.Errorf("%w: status %s", ErrRecoveryNotPending, r.Status)
	}
	r.Status = RecoveryRejected
	return nil
}

// MarkConsumed records that the factor reset has been performed. It is only valid
// from approved — the reset can never precede the peer approvals, so no code path
// resets a factor on an unapproved request.
func (r *RecoveryRequest) MarkConsumed() error {
	if r.Status != RecoveryApproved {
		return fmt.Errorf("%w: status %s", ErrRecoveryNotApproved, r.Status)
	}
	r.Status = RecoveryConsumed
	return nil
}
