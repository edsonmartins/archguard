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
	"crypto/hmac"
	"crypto/sha256"
	"errors"

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
	return &Provisional{deploymentKey: key}, nil
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

// compile-time check that Provisional satisfies the port.
var _ domain.KeyCustodian = (*Provisional)(nil)
