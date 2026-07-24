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

import "time"

// Seal-key rotation with per-key_id validity (pacote 010, T-014 / design 010
// §"Custódia de chaves": "selagem com registro de intervalo de validade por
// key_id"). Each seal signing key VERSION was in force for an interval. A seal is
// verified against the version that produced it (its key_id), and that version
// must have been VALID at the seal's time — a seal claiming a key_id outside its
// validity window is tampering, caught by the verifier (fail-closed).

// SealKeyValidity records the interval a seal key version was in force. NotAfter
// zero means "still current" (the latest version, not yet rotated out).
type SealKeyValidity struct {
	KeyID     string
	NotBefore time.Time
	NotAfter  time.Time
}

// ValidAt reports whether the key was in force at t: t in [NotBefore, NotAfter),
// with an open upper bound when NotAfter is zero.
func (v SealKeyValidity) ValidAt(t time.Time) bool {
	if t.Before(v.NotBefore) {
		return false
	}
	if !v.NotAfter.IsZero() && !t.Before(v.NotAfter) {
		return false
	}
	return true
}

// SealKeyRegistry maps key_id to its validity interval, so the verifier can check
// that a seal's key_id was in force when the seal was produced.
type SealKeyRegistry struct {
	validities map[string]SealKeyValidity
}

// NewSealKeyRegistry builds an empty registry.
func NewSealKeyRegistry() *SealKeyRegistry {
	return &SealKeyRegistry{validities: map[string]SealKeyValidity{}}
}

// Register records (or replaces) a key version's validity. A rotation calls it
// twice: closing the previous version (setting its NotAfter) and opening the new.
func (r *SealKeyRegistry) Register(v SealKeyValidity) {
	r.validities[v.KeyID] = v
}

// ValidForSeal reports whether keyID is a KNOWN key that was in force at sealTime.
// An unknown key_id, or one outside its validity, is refused (fail-closed) — the
// verifier treats it as a tampered or forged seal.
func (r *SealKeyRegistry) ValidForSeal(keyID string, sealTime time.Time) bool {
	v, ok := r.validities[keyID]
	if !ok {
		return false
	}
	return v.ValidAt(sealTime)
}

// Rotate closes the current key (stamping its NotAfter at `at`) and opens the new
// key from `at`. It is the state transition of a seal-key rotation (T-014): after
// it, seals signed by the previous key before `at` still verify (their key_id was
// valid then), and new seals use the new key.
func (r *SealKeyRegistry) Rotate(previousKeyID, newKeyID string, at time.Time) {
	if prev, ok := r.validities[previousKeyID]; ok {
		prev.NotAfter = at
		r.validities[previousKeyID] = prev
	}
	r.validities[newKeyID] = SealKeyValidity{KeyID: newKeyID, NotBefore: at}
}
