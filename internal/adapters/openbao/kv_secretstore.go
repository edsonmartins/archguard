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
	"errors"
	"fmt"
	"strings"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// KVSecretStore implements domain.SecretStore over the OpenBao KV v2 engine
// (pacote 010, T-012). A reversible secret — a client OAuth secret, a directory
// connector's bind credential — is written to the vault and the caller stores only
// the opaque reference this returns (INV-7): the secret never touches the database,
// a log, or a trace. This replaces the provisional (sealed-keystore) SecretStore in
// the production profile.
type KVSecretStore struct {
	client *Client
	mount  string // KV v2 mount, e.g. "secret"
}

// NewKVSecretStore builds the store over a client and a KV v2 mount path.
func NewKVSecretStore(client *Client, mount string) *KVSecretStore {
	return &KVSecretStore{client: client, mount: strings.Trim(mount, "/")}
}

const refPrefix = "openbao:kv:"

// Put writes secret to a fresh vault path and returns an opaque reference. The
// secret is base64-encoded (KV stores JSON strings). Being a vault write (a remote
// call), it MUST NOT run inside a database transaction (RFC-0004 §4).
func (s *KVSecretStore) Put(ctx context.Context, secret []byte) (string, error) {
	path := uuid.NewString()
	body := map[string]any{
		"data": map[string]string{"value": base64.StdEncoding.EncodeToString(secret)},
	}
	if err := s.client.do(ctx, "POST", s.dataPath(path), body, nil); err != nil {
		return "", fmt.Errorf("openbao: gravação do segredo falhou: %w", err)
	}
	return refPrefix + path, nil
}

// Get resolves a reference back to its secret, or domain.ErrSecretNotFound.
func (s *KVSecretStore) Get(ctx context.Context, ref string) ([]byte, error) {
	path, err := s.pathOf(ref)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data struct {
			Data struct {
				Value string `json:"value"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := s.client.do(ctx, "GET", s.dataPath(path), nil, &out); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, domain.ErrSecretNotFound
		}
		return nil, fmt.Errorf("openbao: leitura do segredo falhou: %w", err)
	}
	if out.Data.Data.Value == "" {
		return nil, domain.ErrSecretNotFound
	}
	secret, err := base64.StdEncoding.DecodeString(out.Data.Data.Value)
	if err != nil {
		return nil, fmt.Errorf("openbao: segredo malformado no cofre: %w", err)
	}
	return secret, nil
}

// Delete permanently removes the secret at ref (KV v2 metadata delete). Idempotent:
// deleting an absent reference is not an error (compensation, RFC-0004 §4).
func (s *KVSecretStore) Delete(ctx context.Context, ref string) error {
	path, err := s.pathOf(ref)
	if err != nil {
		return err
	}
	if err := s.client.do(ctx, "DELETE", s.metadataPath(path), nil, nil); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("openbao: remoção do segredo falhou: %w", err)
	}
	return nil
}

func (s *KVSecretStore) dataPath(path string) string { return "/v1/" + s.mount + "/data/" + path }
func (s *KVSecretStore) metadataPath(path string) string {
	return "/v1/" + s.mount + "/metadata/" + path
}

func (s *KVSecretStore) pathOf(ref string) (string, error) {
	if !strings.HasPrefix(ref, refPrefix) {
		return "", fmt.Errorf("openbao: referência inválida %q", ref)
	}
	return strings.TrimPrefix(ref, refPrefix), nil
}

var _ domain.SecretStore = (*KVSecretStore)(nil)
