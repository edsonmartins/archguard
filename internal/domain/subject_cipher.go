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

import "errors"

// SubjectCipher encrypts a data subject's (titular's) personal fields under a
// PER-SUBJECT key (pacote 010, T-018 / ADR-0014). The per-subject key is the unit
// of CRYPTO-SHREDDING: destroying it (DestroySubjectKey) makes every field
// encrypted under it irrecoverable — including in backups, because a backup holds
// only the ciphertext, never the key. That is how LGPD elimination is met without
// deleting rows or breaking the tamper-evident audit chain (the chain keeps only
// the pseudonym). The key MUST live in the vault, never beside the ciphertext.
type SubjectCipher interface {
	// EncryptForSubject encrypts plaintext under subjectID's key, creating the key
	// on first use. Returns ErrSubjectKeyDestroyed if the subject was already
	// eliminated (no new personal data is written for a shredded subject).
	EncryptForSubject(subjectID string, plaintext []byte) (ciphertext []byte, err error)
	// DecryptForSubject decrypts ciphertext. It returns ErrSubjectKeyDestroyed if
	// the key was destroyed (the data is gone) and ErrSubjectKeyMissing if the
	// subject was never encrypted for.
	DecryptForSubject(subjectID string, ciphertext []byte) (plaintext []byte, err error)
	// DestroySubjectKey irreversibly destroys subjectID's key (crypto-shredding).
	// It is idempotent: destroying an already-destroyed or unknown subject succeeds
	// (the end state — no key — is the same).
	DestroySubjectKey(subjectID string) error
}

// Errors of the subject cipher.
var (
	// ErrSubjectKeyDestroyed is returned when an operation targets a subject whose
	// key was crypto-shredded — the personal data is irrecoverable by design.
	ErrSubjectKeyDestroyed = errors.New("subject_cipher: chave do titular destruída — dado irrecuperável (crypto-shredding)")
	// ErrSubjectKeyMissing is returned when decrypting for a subject that was never
	// encrypted for (no key was ever created).
	ErrSubjectKeyMissing = errors.New("subject_cipher: nenhuma chave para o titular")
)
