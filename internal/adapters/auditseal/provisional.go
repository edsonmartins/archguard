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

// Package auditseal holds a PROVISIONAL implementation of the audit sealing
// ports (domain.Sealer / domain.SealVerifier).
//
// This is NOT the production control: in production the seal is signed by the
// OpenBao transit engine and the private key never reaches the application
// (ADR-0012, RFC-0003 §4). This dev-only signer holds Ed25519 keys IN PROCESS so
// the sealing (T-011) and the verifier (T-013) are usable and testable now,
// exactly like the other provisional adapters (globalaccess, tenantswitch). It
// keeps a KEYRING so verification after key rotation (RFC-0003 §4, spec
// scenario) is exercised: old seals verify with the key_id they were signed
// under, new seals with the current key.
package auditseal

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/domain"
)

// Provisional is the in-process Ed25519 signer/verifier. NON-PRODUCTION.
type Provisional struct {
	mu      sync.RWMutex
	keys    map[string]ed25519.PrivateKey // key_id → private key (dev only)
	pubs    map[string]ed25519.PublicKey  // key_id → public key
	current string
	nextN   int
}

// NewProvisional builds the provisional signer with one freshly generated key.
func NewProvisional() (*Provisional, error) {
	p := &Provisional{keys: map[string]ed25519.PrivateKey{}, pubs: map[string]ed25519.PublicKey{}}
	if _, err := p.Rotate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Rotate generates a new key, makes it current, and returns its key_id. Older
// keys stay in the keyring so seals signed under them remain verifiable.
func (p *Provisional) Rotate() (string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("auditseal: geração da chave Ed25519 falhou: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextN++
	keyID := fmt.Sprintf("provisional-ed25519-%d", p.nextN)
	p.keys[keyID] = priv
	p.pubs[keyID] = pub
	p.current = keyID
	return keyID, nil
}

// Sign signs content with the current key, returning the signature and its
// key_id (RFC-0003 §4).
func (p *Provisional) Sign(_ context.Context, content []byte) ([]byte, string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	priv, ok := p.keys[p.current]
	if !ok {
		return nil, "", fmt.Errorf("auditseal: sem chave de assinatura corrente")
	}
	return ed25519.Sign(priv, content), p.current, nil
}

// Verify checks the signature against the public key matching keyID. An unknown
// key_id is fail-closed (domain.ErrSealKeyUnknown).
func (p *Provisional) Verify(_ context.Context, content, signature []byte, keyID string) (bool, error) {
	p.mu.RLock()
	pub, ok := p.pubs[keyID]
	p.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("%w: %s", domain.ErrSealKeyUnknown, keyID)
	}
	return ed25519.Verify(pub, content, signature), nil
}

// PublicKey returns the public key for keyID, for export to an external verifier
// (RFC-0003 §9 / T-016). Returns nil if unknown.
func (p *Provisional) PublicKey(keyID string) ed25519.PublicKey {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pubs[keyID]
}

var (
	_ domain.Sealer       = (*Provisional)(nil)
	_ domain.SealVerifier = (*Provisional)(nil)
)
