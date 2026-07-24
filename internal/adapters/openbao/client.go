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

// Package openbao is the HTTP client for OpenBao (pacote 010, ADR-0012). OpenBao
// is reached over HTTP and NEVER linked into the binary (MPL-2.0 boundary,
// INV-4): this package speaks its REST API with the standard library only — no
// vendor SDK, no new dependency. It backs the real key custody and secret store
// (T-010/011/012) so that private keys and reversible secrets live in the vault,
// and the database holds only references (INV-7).
package openbao

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal OpenBao HTTP client (KV v2 + transit). It carries the vault
// address and the auth token; the token is a SECRET and is never logged.
type Client struct {
	addr  string
	token string
	http  *http.Client
}

// New builds a client for the vault at addr, authenticating with token. addr is
// like "https://openbao.internal:8200". A default 5s HTTP timeout is applied so a
// hung vault fails fast (fail-closed callers deny on error).
func New(addr, token string) *Client {
	return &Client{
		addr:  strings.TrimRight(addr, "/"),
		token: token,
		http:  &http.Client{Timeout: 5 * time.Second},
	}
}

// NewWithHTTP builds a client with a caller-supplied http.Client (tests inject a
// client pointed at a fake server).
func NewWithHTTP(addr, token string, hc *http.Client) *Client {
	return &Client{addr: strings.TrimRight(addr, "/"), token: token, http: hc}
}

// do issues a request to the vault API path (e.g. "/v1/secret/data/x"), sending
// body as JSON when non-nil, and decodes a JSON response into out when non-nil.
// It returns ErrNotFound for 404 and a generic error for other non-2xx statuses —
// never echoing the response body, which could carry secret material.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("openbao: serialização do corpo falhou: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.addr+path, reader)
	if err != nil {
		return fmt.Errorf("openbao: montagem da requisição falhou: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("openbao: chamada ao cofre falhou: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		// Deliberately do NOT include the body — it may carry secret material.
		return fmt.Errorf("openbao: cofre respondeu status %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("openbao: decodificação da resposta falhou: %w", err)
		}
	}
	return nil
}

// ErrNotFound is returned when a vault path resolves to nothing (404).
var ErrNotFound = fmt.Errorf("openbao: caminho não encontrado no cofre")
