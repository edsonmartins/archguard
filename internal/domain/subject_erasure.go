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

	"github.com/google/uuid"
)

// Subject erasure (pacote 010, T-019 / ADR-0014 / spec "Eliminação por destruição
// de chave"). Eliminating a titular's personal data is CRYPTO-SHREDDING: the
// subject's key is destroyed, the ciphertext (even in backups) becomes garbage,
// and the audit chain stays verifiable because it only ever held the pseudonym.
// The act is itself an L3 operation and an audit event, and it demands an EXPLICIT
// acknowledgement of irreversibility.

// ErasureRequest is an L3-confirmed request to erase a titular.
type ErasureRequest struct {
	// SubjectID is the titular's opaque pseudonym (identity.Subject) — the
	// crypto-shredding key id. Never an e-mail.
	SubjectID string
	// OrganizationID is the tenant context recorded on the audit event.
	OrganizationID uuid.UUID
	// OperatorSubject is who triggered the erasure (audit actor, pseudonym).
	OperatorSubject string
	// AcknowledgedIrreversible is the operator's explicit confirmation that the
	// erasure is irreversible. Without it the erasure is refused.
	AcknowledgedIrreversible bool
}

// Errors of subject erasure.
var (
	ErrErasureSubjectRequired = errors.New("erasure: titular obrigatório")
	// ErrErasureNotAcknowledged is returned when the irreversibility was not
	// explicitly acknowledged — the confirmation the spec requires.
	ErrErasureNotAcknowledged = errors.New("erasure: eliminação exige confirmação explícita da irreversibilidade")
)

// Validate refuses an erasure with no subject or no irreversibility acknowledgement.
func (r ErasureRequest) Validate() error {
	if r.SubjectID == "" {
		return ErrErasureSubjectRequired
	}
	if !r.AcknowledgedIrreversible {
		return ErrErasureNotAcknowledged
	}
	return nil
}

// BuildErasureAuditInput builds the audit event for an erasure (subject.erasure,
// L3). It carries only pseudonyms — the erased subject and the operator — never
// personal data, so recording it does not re-introduce what was just shredded.
func BuildErasureAuditInput(r ErasureRequest) AuditEventInput {
	return AuditEventInput{
		OrganizationID: r.OrganizationID,
		Action:         ActionSubjectErasure,
		Actor:          AuditActor{IdentitySubject: r.OperatorSubject},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "subject", ID: r.SubjectID, Label: "eliminação de titular (crypto-shredding)"},
		Reason:         "eliminação irreversível dos dados pessoais do titular por destruição da chave (ADR-0014)",
		Context:        AuditContext{AuthContextClass: "L3"},
	}
}

// EraseSubject performs the crypto-shredding: it validates the L3 confirmation and
// destroys the subject's key, returning the audit event the caller MUST record.
// DestroySubjectKey is idempotent, so a caller that records the audit and then
// retries the destroy on failure converges safely. Returns an error (and no audit
// input) if the request is unconfirmed or the destroy fails.
func EraseSubject(cipher SubjectCipher, req ErasureRequest) (AuditEventInput, error) {
	if err := req.Validate(); err != nil {
		return AuditEventInput{}, err
	}
	if err := cipher.DestroySubjectKey(req.SubjectID); err != nil {
		return AuditEventInput{}, err
	}
	return BuildErasureAuditInput(req), nil
}
