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
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// SealAnchor writes a seal to an EXTERNAL write-once (WORM) destination — the
// optional external anchor of RFC-0003 §4. Anchoring the signed seals to a
// destination the instance cannot rewrite (S3 Object Lock, an on-prem immutable
// store) makes tampering detectable even if the whole ArchGuard instance is
// compromised: an attacker who rewrites the local chain and forges seals still
// cannot alter the copies already anchored externally. Anchor returns an opaque
// reference to the stored object; Fetch reads it back for verification.
type SealAnchor interface {
	Anchor(ctx context.Context, seal Seal) (ref string, err error)
	Fetch(ctx context.Context, ref string) (Seal, error)
}

// SealExport is the wire form of a seal written to the WORM destination — a
// self-contained, verifiable record (the range, head, time, key_id and
// signature). head_hash and signature travel as hex so the object is plain
// text/JSON, portable to any external store and re-verifiable offline.
type SealExport struct {
	OrganizationID string `json:"organization_id"`
	SeqStart       int64  `json:"seq_start"`
	SeqEnd         int64  `json:"seq_end"`
	HeadHashHex    string `json:"head_hash_hex"`
	SealedAtMicros int64  `json:"sealed_at_us"`
	KeyID          string `json:"key_id"`
	SignatureHex   string `json:"signature_hex"`
}

// MarshalSealExport renders a seal as the portable WORM object bytes.
func MarshalSealExport(seal Seal) ([]byte, error) {
	exp := SealExport{
		OrganizationID: seal.OrganizationID.String(),
		SeqStart:       seal.SeqStart,
		SeqEnd:         seal.SeqEnd,
		HeadHashHex:    fmt.Sprintf("%x", seal.HeadHash),
		SealedAtMicros: seal.SealedAt,
		KeyID:          seal.KeyID,
		SignatureHex:   fmt.Sprintf("%x", seal.Signature),
	}
	b, err := json.Marshal(exp)
	if err != nil {
		return nil, fmt.Errorf("audit: serialização do selo para WORM falhou: %w", err)
	}
	return b, nil
}

// UnmarshalSealExport reconstructs a Seal from the portable WORM object bytes,
// so an anchored seal can be verified against the local one (or offline).
func UnmarshalSealExport(b []byte) (Seal, error) {
	var exp SealExport
	if err := json.Unmarshal(b, &exp); err != nil {
		return Seal{}, fmt.Errorf("audit: desserialização do selo WORM falhou: %w", err)
	}
	orgID, err := uuid.Parse(exp.OrganizationID)
	if err != nil {
		return Seal{}, fmt.Errorf("audit: organization_id do selo WORM inválido: %w", err)
	}
	headHash, err := hex.DecodeString(exp.HeadHashHex)
	if err != nil {
		return Seal{}, fmt.Errorf("audit: head_hash do selo WORM inválido: %w", err)
	}
	signature, err := hex.DecodeString(exp.SignatureHex)
	if err != nil {
		return Seal{}, fmt.Errorf("audit: signature do selo WORM inválido: %w", err)
	}
	return Seal{
		OrganizationID: orgID,
		SeqStart:       exp.SeqStart,
		SeqEnd:         exp.SeqEnd,
		HeadHash:       headHash,
		SealedAt:       exp.SealedAtMicros,
		KeyID:          exp.KeyID,
		Signature:      signature,
	}, nil
}
