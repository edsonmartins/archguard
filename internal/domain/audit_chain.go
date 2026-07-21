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
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Chain constants (RFC-0003 §3).
const (
	// AuditHashSize is the length in bytes of a chain hash (SHA-256).
	AuditHashSize = sha256.Size
	// AuditGenesisNonceSize is the length of the per-organization genesis nonce.
	// The nonce is random material generated once per chain (per organization),
	// so two tenants' chains cannot share a genesis even with the same id.
	AuditGenesisNonceSize = 32
)

// Errors of the chain primitives.
var (
	// ErrInvalidPrevHash is returned when a previous hash is not exactly
	// AuditHashSize bytes — a malformed chain link, never silently accepted.
	ErrInvalidPrevHash = errors.New("audit: prev_hash inválido (tamanho incorreto)")
	// ErrInvalidGenesisNonce is returned when the genesis nonce is not exactly
	// AuditGenesisNonceSize bytes.
	ErrInvalidGenesisNonce = errors.New("audit: genesis_nonce inválido (tamanho incorreto)")
	// ErrInvalidSeq is returned when a sequence is not strictly positive (seq 0
	// is the genesis sentinel, never a real event).
	ErrInvalidSeq = errors.New("audit: seq inválido (deve ser ≥ 1)")
)

// GenesisHash is hash(0) of an organization's chain (RFC-0003 §3):
// H(organization_id || genesis_nonce). It anchors the chain so the first real
// event links to something tenant-specific and unforgeable. The nonce is
// generated once per organization and stored with the chain head (T-004).
func GenesisHash(organizationID uuid.UUID, genesisNonce []byte) ([]byte, error) {
	if organizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organização obrigatória", ErrInvalidAuditEvent)
	}
	if len(genesisNonce) != AuditGenesisNonceSize {
		return nil, ErrInvalidGenesisNonce
	}
	h := sha256.New()
	orgBytes := organizationID // uuid.UUID is [16]byte
	h.Write(orgBytes[:])
	h.Write(genesisNonce)
	return h.Sum(nil), nil
}

// SealEvent links one event into the chain (RFC-0003 §3): it canonicalizes the
// event and computes hash = H(prev_hash || canonical), returning the SealedEvent
// with the caller-assigned seq. The seq is assigned by the write path (T-004),
// which serializes per organization so the sequence is gapless and race-free;
// this function only enforces that it is a real (≥1) sequence and that prev_hash
// is a well-formed 32-byte link.
//
// The concatenation is unambiguous because prev_hash is fixed-length
// (AuditHashSize): the first 32 bytes are always the previous hash and the rest
// is the canonical content, so no length delimiter is needed.
func SealEvent(event AuditEvent, prevHash []byte, seq int64) (SealedEvent, error) {
	if len(prevHash) != AuditHashSize {
		return SealedEvent{}, ErrInvalidPrevHash
	}
	if seq < 1 {
		return SealedEvent{}, ErrInvalidSeq
	}
	canonical, err := Canonical(event)
	if err != nil {
		return SealedEvent{}, err
	}
	h := sha256.New()
	h.Write(prevHash)
	h.Write(canonical)
	return SealedEvent{
		Event:    event,
		Seq:      seq,
		PrevHash: prevHash,
		Hash:     h.Sum(nil),
	}, nil
}

// VerifyLink recomputes the hash of a sealed event from its prev_hash and
// content and reports whether it matches the stored hash — the per-link check
// the full verifier (T-013) runs in sequence over a chain. A mismatch means the
// event's content or its prev_hash was altered.
func VerifyLink(sealed SealedEvent) (bool, error) {
	recomputed, err := SealEvent(sealed.Event, sealed.PrevHash, max64(sealed.Seq, 1))
	if err != nil {
		return false, err
	}
	return bytesEqual(recomputed.Hash, sealed.Hash), nil
}

// bytesEqual is a length-checked byte comparison (constant-time not required:
// these are public hashes, not secrets).
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
