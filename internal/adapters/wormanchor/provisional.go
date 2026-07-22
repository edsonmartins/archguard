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

// Package wormanchor holds a PROVISIONAL implementation of the seal-anchoring
// port (domain.SealAnchor).
//
// This is NOT the production control: in production the seals are anchored to an
// external write-once store the instance cannot rewrite (S3 Object Lock, an
// on-prem immutable store). This dev-only anchor keeps the objects IN MEMORY,
// write-once (an existing ref cannot be overwritten — emulating Object Lock), so
// the export (T-012) and its verification are usable and testable now, like the
// other provisional adapters. It is NOT durable (lost on restart), which is fine
// for dev/CI but never for production.
package wormanchor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/domain"
)

// ErrWORMOverwrite is returned when a write targets a ref that already holds a
// DIFFERENT object — the write-once guarantee (a real WORM store refuses this).
var ErrWORMOverwrite = errors.New("wormanchor: destino WORM é write-once — sobrescrita recusada")

// Memory is the in-memory write-once anchor. NON-PRODUCTION.
type Memory struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMemory builds the provisional in-memory WORM anchor.
func NewMemory() *Memory {
	return &Memory{objects: map[string][]byte{}}
}

// Anchor writes the seal's portable form to the store under a content-addressed
// ref (the SHA-256 of the object). Content addressing makes the write naturally
// idempotent: anchoring the same seal twice targets the same ref with identical
// bytes (allowed); a ref holding DIFFERENT bytes is refused (ErrWORMOverwrite).
func (m *Memory) Anchor(_ context.Context, seal domain.Seal) (string, error) {
	obj, err := domain.MarshalSealExport(seal)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(obj)
	ref := "worm://" + hex.EncodeToString(sum[:])

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.objects[ref]; ok {
		if !bytesEqual(existing, obj) {
			return "", fmt.Errorf("%w: %s", ErrWORMOverwrite, ref)
		}
		return ref, nil // already anchored, identical bytes — idempotent
	}
	stored := make([]byte, len(obj))
	copy(stored, obj)
	m.objects[ref] = stored
	return ref, nil
}

// Fetch reads back the anchored seal at ref for verification.
func (m *Memory) Fetch(_ context.Context, ref string) (domain.Seal, error) {
	m.mu.RLock()
	obj, ok := m.objects[ref]
	m.mu.RUnlock()
	if !ok {
		return domain.Seal{}, fmt.Errorf("wormanchor: ref não encontrada: %s", ref)
	}
	return domain.UnmarshalSealExport(obj)
}

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

var _ domain.SealAnchor = (*Memory)(nil)
