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

// Package keycustodian holds implementations of the domain.KeyCustodian port.
//
// The Provisional implementation here is NOT SUPPORTED IN PRODUCTION: it keeps
// the deployment key in process memory with no rotation, no HSM/OpenBao custody
// and no key-versioning. It exists so identity multi-tenancy can be built and
// tested end to end before full key management lands (package 010, ADR-0012).
// The `production` deployment profile MUST wire the port to OpenBao instead; the
// dev profile may use this backed by material from the sealed keystore.
package keycustodian

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/domain"
)

// minDeploymentKeyBytes is the minimum accepted deployment-key length: 256 bits,
// matching the HMAC-SHA256 output. A shorter key is rejected rather than
// silently weakening the MAC.
const minDeploymentKeyBytes = 32

// ErrWeakDeploymentKey is returned when the provided deployment key is shorter
// than minDeploymentKeyBytes.
var ErrWeakDeploymentKey = errors.New("keycustodian: chave de deployment fraca (< 256 bits)")

// Provisional is an in-memory HMAC-SHA256 key custodian. See the package doc:
// it is a development/test stand-in for the real OpenBao-backed custodian and is
// not supported in production.
type Provisional struct {
	deploymentKey []byte

	// Per-subject cipher state (T-018). subjectKeys holds each titular's AES-256
	// key; destroyed marks crypto-shredded subjects. In production these live in
	// the vault; here they are in-process (dev/test only).
	mu          sync.Mutex
	subjectKeys map[string][]byte
	destroyed   map[string]bool
}

// NewProvisional builds a Provisional custodian from a deployment key. The key
// is defensively copied so the caller cannot mutate it afterwards. It requires
// at least a 256-bit key (ErrWeakDeploymentKey otherwise).
func NewProvisional(deploymentKey []byte) (*Provisional, error) {
	if len(deploymentKey) < minDeploymentKeyBytes {
		return nil, ErrWeakDeploymentKey
	}
	key := make([]byte, len(deploymentKey))
	copy(key, deploymentKey)
	return &Provisional{
		deploymentKey: key,
		subjectKeys:   map[string][]byte{},
		destroyed:     map[string]bool{},
	}, nil
}

// EncryptForSubject implements domain.SubjectCipher: AES-256-GCM under the
// subject's key (created on first use), nonce prepended. A shredded subject is
// refused (ErrSubjectKeyDestroyed) — no new personal data for an eliminated titular.
func (p *Provisional) EncryptForSubject(subjectID string, plaintext []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.destroyed[subjectID] {
		return nil, domain.ErrSubjectKeyDestroyed
	}
	key, ok := p.subjectKeys[subjectID]
	if !ok {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("keycustodian: geração de chave do titular falhou: %w", err)
		}
		p.subjectKeys[subjectID] = key
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("keycustodian: geração de nonce falhou: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptForSubject implements domain.SubjectCipher. A destroyed key yields
// ErrSubjectKeyDestroyed; an unknown subject yields ErrSubjectKeyMissing.
func (p *Provisional) DecryptForSubject(subjectID string, ciphertext []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.destroyed[subjectID] {
		return nil, domain.ErrSubjectKeyDestroyed
	}
	key, ok := p.subjectKeys[subjectID]
	if !ok {
		return nil, domain.ErrSubjectKeyMissing
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("keycustodian: ciphertext curto demais")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("keycustodian: decifragem do titular falhou: %w", err)
	}
	return pt, nil
}

// DestroySubjectKey implements domain.SubjectCipher: it zeroes and removes the
// subject's key and tombstones the subject. Idempotent. After this, every field
// encrypted under it is irrecoverable (crypto-shredding, ADR-0014).
func (p *Provisional) DestroySubjectKey(subjectID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if key, ok := p.subjectKeys[subjectID]; ok {
		for i := range key {
			key[i] = 0
		}
		delete(p.subjectKeys, subjectID)
	}
	p.destroyed[subjectID] = true
	return nil
}

// newGCM builds an AES-256-GCM AEAD from a 32-byte key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("keycustodian: cipher AES falhou: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("keycustodian: GCM falhou: %w", err)
	}
	return gcm, nil
}

// HashEmail implements domain.KeyCustodian: it normalizes the e-mail and returns
// HMAC-SHA256(deploymentKey, normalized). It returns domain.ErrEmptyEmail when
// the address normalizes to empty (a nil hash, never the hash of "").
func (p *Provisional) HashEmail(email string) ([]byte, error) {
	norm := domain.NormalizeEmail(email)
	if norm == "" {
		return nil, domain.ErrEmptyEmail
	}
	mac := hmac.New(sha256.New, p.deploymentKey)
	mac.Write([]byte(norm))
	return mac.Sum(nil), nil
}

// compile-time checks that Provisional satisfies the ports.
var (
	_ domain.KeyCustodian  = (*Provisional)(nil)
	_ domain.SubjectCipher = (*Provisional)(nil)
)
