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

// Package keystore is the sealed local key custodian for the `dev` deployment
// profile (ADR-0017 §3). Private keys live encrypted (AES-256-GCM) in a file
// OUTSIDE the database — the database holds only a reference (I-4.3). The
// sealing material is supplied at boot (never persisted with the keystore, never
// in the database, never a default). Without it the process cannot open the
// keystore: there is no silent auto-generation.
//
// This is explicitly NOT a production custodian: pilot/production use OpenBao
// (ADR-0012). The dev keystore exists so development, CI and the smoke test do
// not need OpenBao while still keeping keys off the database in the clear.
package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrNoSealingMaterial is returned when the keystore is opened without sealing
// material — the process must refuse to start (ADR-0017 §3).
var ErrNoSealingMaterial = errors.New("keystore: sealing material is required and must be provided at boot")

// SealedKeystore holds named private keys, encrypted at rest under a key derived
// from the boot sealing material. It is safe for concurrent use.
type SealedKeystore struct {
	path string
	aead cipher.AEAD
	mu   sync.RWMutex
	keys map[string]string // name -> PEM private key (plaintext in memory only)
}

// Open unseals (or initializes) the keystore file at path using material. An
// empty material is refused. A tampered or wrong-material file fails to decrypt
// (AES-GCM authentication), so a bad unseal never silently yields an empty or
// corrupt keystore.
func Open(path string, material []byte) (*SealedKeystore, error) {
	if len(material) == 0 {
		return nil, ErrNoSealingMaterial
	}
	// Derive a 32-byte AES-256 key from the material. SHA-256 accepts material of
	// any length while fixing the key size.
	sum := sha256.Sum256(material)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ks := &SealedKeystore{path: path, aead: aead, keys: map[string]string{}}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ks, nil // fresh keystore, nothing sealed yet
	}
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize() {
		return nil, fmt.Errorf("keystore: arquivo selado truncado")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("keystore: unseal falhou — material incorreto ou arquivo adulterado: %w", err)
	}
	if err := json.Unmarshal(plaintext, &ks.keys); err != nil {
		return nil, fmt.Errorf("keystore: conteúdo selado ilegível: %w", err)
	}
	return ks, nil
}

// Get returns the PEM private key stored under name.
func (k *SealedKeystore) Get(name string) (string, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	pem, ok := k.keys[name]
	return pem, ok
}

// Has reports whether a key is stored under name.
func (k *SealedKeystore) Has(name string) bool {
	_, ok := k.Get(name)
	return ok
}

// Put seals the PEM private key under name and persists the keystore atomically.
func (k *SealedKeystore) Put(name, privateKeyPEM string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys[name] = privateKeyPEM
	return k.persistLocked()
}

// Delete removes the key stored under name and persists the keystore
// atomically. It is idempotent: deleting an absent name is a no-op success.
func (k *SealedKeystore) Delete(name string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.keys[name]; !ok {
		return nil
	}
	delete(k.keys, name)
	return k.persistLocked()
}

// persistLocked encrypts the whole key map and writes it atomically. Caller
// holds the write lock.
func (k *SealedKeystore) persistLocked() error {
	plaintext, err := json.Marshal(k.keys)
	if err != nil {
		return err
	}
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := k.aead.Seal(nonce, nonce, plaintext, nil)

	if dir := filepath.Dir(k.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := k.path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, k.path)
}
