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
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/casdoor/casdoor/internal/domain"
)

// TransitSigner implements domain.VaultSigner over the OpenBao transit engine
// (pacote 010, T-010/T-011). The signing key is generated and held in the transit
// engine; the application asks the vault to SIGN and only ever reads the PUBLIC
// key — the private key never leaves the vault (ADR-0012). It backs both JWKS token
// signing and audit-seal signing (different key names).
type TransitSigner struct {
	client *Client
	mount  string // transit mount, e.g. "transit"
}

// NewTransitSigner builds the signer over a client and a transit mount path.
func NewTransitSigner(client *Client, mount string) *TransitSigner {
	return &TransitSigner{client: client, mount: strings.Trim(mount, "/")}
}

// Sign asks the vault to sign data under keyName and returns the raw signature.
func (s *TransitSigner) Sign(ctx context.Context, keyName string, data []byte) ([]byte, error) {
	sig, _, err := s.signVersioned(ctx, keyName, data)
	return sig, err
}

// signVersioned signs data and returns the raw signature and the KEY VERSION the
// vault used. The version is what a seal keeps as part of its key_id, so a seal
// produced before a rotation is verified against the right key later (T-014).
func (s *TransitSigner) signVersioned(ctx context.Context, keyName string, data []byte) ([]byte, int, error) {
	body := map[string]string{"input": base64.StdEncoding.EncodeToString(data)}
	var out struct {
		Data struct {
			Signature string `json:"signature"`
		} `json:"data"`
	}
	if err := s.client.do(ctx, "POST", "/v1/"+s.mount+"/sign/"+keyName, body, &out); err != nil {
		return nil, 0, fmt.Errorf("openbao: assinatura no cofre falhou: %w", err)
	}
	sig := out.Data.Signature
	if sig == "" {
		return nil, 0, fmt.Errorf("openbao: cofre não retornou assinatura")
	}
	// Format is "vault:v<version>:<base64>".
	parts := strings.SplitN(sig, ":", 3)
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "v") {
		return nil, 0, fmt.Errorf("openbao: formato de assinatura inesperado")
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[1], "v"))
	if err != nil {
		return nil, 0, fmt.Errorf("openbao: versão de chave inválida na assinatura: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, 0, fmt.Errorf("openbao: assinatura malformada do cofre: %w", err)
	}
	return raw, version, nil
}

// PublicKey returns the PEM of keyName's LATEST public key. The private key is
// never fetched — the vault does not expose it.
func (s *TransitSigner) PublicKey(ctx context.Context, keyName string) ([]byte, error) {
	var out struct {
		Data struct {
			LatestVersion int `json:"latest_version"`
			Keys          map[string]struct {
				PublicKey string `json:"public_key"`
			} `json:"keys"`
		} `json:"data"`
	}
	if err := s.client.do(ctx, "GET", "/v1/"+s.mount+"/keys/"+keyName, nil, &out); err != nil {
		return nil, fmt.Errorf("openbao: leitura da chave pública falhou: %w", err)
	}
	version := strconv.Itoa(out.Data.LatestVersion)
	k, ok := out.Data.Keys[version]
	if !ok || k.PublicKey == "" {
		return nil, fmt.Errorf("openbao: chave pública %q versão %s ausente", keyName, version)
	}
	return []byte(k.PublicKey), nil
}

var _ domain.VaultSigner = (*TransitSigner)(nil)
