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
	"time"

	"github.com/google/uuid"
)

// Key rotation policy (pacote 010, T-013 / design 010 §"Custódia de chaves"). A
// JWKS rotation keeps the PREVIOUS signing key published for an OVERLAP window, so
// tokens issued under it stay verifiable until they expire (spec "tokens emitidos
// anteriormente permanecem válidos até expirar"). The overlap MUST be at least the
// maximum token TTL — otherwise a live token would be orphaned by the rotation.
// Every rotation is an L3 operation and an audit event (spec "Autorização da
// rotação": exige L3 e é registrada).

// ErrOverlapTooShort is returned when a JWKS rotation's overlap window is shorter
// than the maximum token TTL — the rotation would orphan live tokens.
var ErrOverlapTooShort = errors.New("key_rotation: janela de sobreposição menor que o TTL máximo de token")

// ValidateRotationOverlap ensures the JWKS overlap is at least the max token TTL.
func ValidateRotationOverlap(overlap, maxTokenTTL time.Duration) error {
	if overlap < maxTokenTTL {
		return fmt.Errorf("%w: sobreposição %s < TTL %s", ErrOverlapTooShort, overlap, maxTokenTTL)
	}
	return nil
}

// BuildKeyRotationAuditInput builds the audit event for a key rotation (key.rotate,
// L3). It records who rotated and which key (by non-personal key id), with the
// assurance context L3.
func BuildKeyRotationAuditInput(organizationID uuid.UUID, operatorSubject, keyID, keyKind string) AuditEventInput {
	return AuditEventInput{
		OrganizationID: organizationID,
		Action:         ActionKeyRotate,
		Actor:          AuditActor{IdentitySubject: operatorSubject},
		Outcome:        Allowed,
		Target:         AuditTarget{Type: "key", ID: keyID, Label: "rotação de chave (" + keyKind + ")"},
		Reason:         "rotação de chave " + keyKind + " com sobreposição preservando operações em curso",
		Context:        AuditContext{AuthContextClass: "L3"},
	}
}
