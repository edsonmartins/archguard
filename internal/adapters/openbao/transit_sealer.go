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

package openbao

import (
	"context"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
)

// TransitSealer implements domain.Sealer over the OpenBao transit engine (pacote
// 010, T-011 / RFC-0003 §4): audit seals are signed IN THE VAULT with the custodied
// Ed25519 seal key — the private key never reaches the application (ADR-0012).
// The key_id it returns is "<keyName>:v<version>", so a seal produced before a key
// rotation is later verified against the exact version that signed it (T-014).
type TransitSealer struct {
	signer  *TransitSigner
	keyName string
}

// NewTransitSealer builds the sealer over a client, the transit mount, and the seal
// key name.
func NewTransitSealer(client *Client, mount, keyName string) *TransitSealer {
	return &TransitSealer{signer: NewTransitSigner(client, mount), keyName: keyName}
}

// Sign signs seal content in the vault and returns the signature and the versioned
// key_id. A vault failure surfaces as an error — the seal is not produced, and the
// caller (audit sealing) fails closed.
func (s *TransitSealer) Sign(ctx context.Context, content []byte) ([]byte, string, error) {
	sig, version, err := s.signer.signVersioned(ctx, s.keyName, content)
	if err != nil {
		return nil, "", fmt.Errorf("openbao: selagem no cofre falhou: %w", err)
	}
	return sig, fmt.Sprintf("%s:v%d", s.keyName, version), nil
}

var _ domain.Sealer = (*TransitSealer)(nil)
