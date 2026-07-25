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
	"strings"

	"github.com/casdoor/casdoor/internal/domain"
)

// TransitCustodian implements domain.KeyCustodian over the OpenBao transit engine:
// the e-mail hash is an HMAC-SHA256 computed BY THE VAULT under a deployment key
// that never leaves it (INV-7 / ADR-0012). It is the conformant-profile replacement
// for keycustodian.Provisional (which keeps the key in process memory).
//
// The hash must be STABLE for dedup (RFC-0002): HMAC is deterministic, and the
// transit HMAC uses the key's LATEST version — so the e-mail-hash key MUST NOT be
// rotated (provision it with rotation disabled). Rotating it would change every
// e-mail hash and break identity deduplication.
type TransitCustodian struct {
	client  *Client
	mount   string // transit mount, e.g. "transit"
	keyName string // the non-rotating e-mail-hash key
}

// NewTransitCustodian builds the custodian over a client, the transit mount and the
// e-mail-hash key name.
func NewTransitCustodian(client *Client, mount, keyName string) *TransitCustodian {
	return &TransitCustodian{client: client, mount: strings.Trim(mount, "/"), keyName: keyName}
}

// HashEmail implements domain.KeyCustodian: it normalizes the e-mail and returns
// HMAC-SHA256(deploymentKey, normalized), computed in the vault. An address that
// normalizes to empty yields domain.ErrEmptyEmail (a nil hash, never the hash of
// ""). A vault failure is an error (fail-closed, INV-6).
func (c *TransitCustodian) HashEmail(email string) ([]byte, error) {
	norm := domain.NormalizeEmail(email)
	if norm == "" {
		return nil, domain.ErrEmptyEmail
	}
	body := map[string]string{
		"input":     base64.StdEncoding.EncodeToString([]byte(norm)),
		"algorithm": "sha2-256",
	}
	var out struct {
		Data struct {
			HMAC string `json:"hmac"`
		} `json:"data"`
	}
	// The interface is context-free; the client's HTTP timeout bounds the call.
	if err := c.client.do(context.Background(), "POST", "/v1/"+c.mount+"/hmac/"+c.keyName, body, &out); err != nil {
		return nil, fmt.Errorf("openbao: HMAC de e-mail no cofre falhou: %w", err)
	}
	// Format is "vault:v<version>:<base64>".
	parts := strings.SplitN(out.Data.HMAC, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("openbao: formato de HMAC inesperado do cofre")
	}
	raw, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("openbao: HMAC malformado do cofre: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("openbao: cofre retornou HMAC vazio")
	}
	return raw, nil
}

var _ domain.KeyCustodian = (*TransitCustodian)(nil)
