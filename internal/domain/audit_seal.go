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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// A seal binds a range of the chain to a signed head (RFC-0003 §4): to alter a
// sealed event undetectably, an attacker would have to forge the signature,
// which the vault-held key makes infeasible. The seal is what makes RETROACTIVE
// tampering detectable even by someone with database access.
type Seal struct {
	OrganizationID uuid.UUID
	SeqStart       int64
	SeqEnd         int64
	HeadHash       []byte
	SealedAt       int64 // microseconds since epoch (UTC), stable for signing
	// KeyID identifies the signing key so a seal stays verifiable after the key
	// is rotated (RFC-0003 §4). It is NOT part of the signed content — the
	// verifier only trusts key_ids in its keyring, so it cannot be swapped for
	// an attacker-controlled key.
	KeyID     string
	Signature []byte
}

// Errors of the sealing path.
var (
	// ErrInvalidSeal is returned when a seal's range or head is malformed.
	ErrInvalidSeal = errors.New("audit: selo inválido")
	// ErrSealKeyUnknown is returned by a verifier asked to verify with a key_id
	// it does not hold — fail-closed, an unknown key never verifies.
	ErrSealKeyUnknown = errors.New("audit: key_id de selo desconhecido")
)

// SealContent produces the deterministic bytes that get SIGNED for a seal — the
// range, the head hash and the sealing time, canonicalized (sorted keys, head
// hash as hex, times as integer microseconds). key_id and signature are
// excluded. A verifier recomputes exactly these bytes from the stored seal and
// checks the signature, so any change to the range, head or time breaks it.
func SealContent(organizationID uuid.UUID, seqStart, seqEnd int64, headHash []byte, sealedAtMicros int64) ([]byte, error) {
	if organizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organização obrigatória", ErrInvalidSeal)
	}
	if seqStart < 1 || seqEnd < seqStart {
		return nil, fmt.Errorf("%w: intervalo de seq inválido [%d,%d]", ErrInvalidSeal, seqStart, seqEnd)
	}
	if len(headHash) != AuditHashSize {
		return nil, fmt.Errorf("%w: head_hash com tamanho incorreto", ErrInvalidSeal)
	}
	m := map[string]any{
		"organization_id": organizationID.String(),
		"seq_start":       seqStart,
		"seq_end":         seqEnd,
		"head_hash":       fmt.Sprintf("%x", headHash),
		"sealed_at_us":    sealedAtMicros,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("audit: canonicalização do selo falhou: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Content returns the signable bytes of this seal.
func (s Seal) Content() ([]byte, error) {
	return SealContent(s.OrganizationID, s.SeqStart, s.SeqEnd, s.HeadHash, s.SealedAt)
}

// Sealer signs seal content with the custodied signing key and returns the
// signature and the key_id used. In production this is the OpenBao transit
// engine — the private key never reaches the application (ADR-0012, RFC-0003
// §4). The provisional dev implementation holds a local Ed25519 key.
type Sealer interface {
	Sign(ctx context.Context, content []byte) (signature []byte, keyID string, err error)
}

// SealVerifier verifies a signature against the public key matching key_id.
// key_id lets it verify seals produced before a key rotation. Fail-closed: an
// unknown key_id or a bad signature returns (false, ...), never a pass.
type SealVerifier interface {
	Verify(ctx context.Context, content, signature []byte, keyID string) (bool, error)
}
