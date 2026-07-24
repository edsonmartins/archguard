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

package postgres

import (
	"context"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// DirectoryConnectorProvisioner creates a directory connector whose bind
// credential is CUSTODIED IN THE VAULT (pacote 009, T-006 / RFC-0007 §5.1,
// ADR-0012). The bind secret is written to the SecretStore and only its opaque
// reference reaches the database (INV-7): the credential never touches a table, a
// log, or a trace. It also resolves the credential back for the syncer.
//
// Ordering follows RFC-0004 §4: the vault write (a remote call) happens OUTSIDE
// the database transaction, and if the row cannot be persisted the orphaned vault
// entry is compensated (deleted), so a vaulted secret never outlives its row.
type DirectoryConnectorProvisioner struct {
	repo    *TenantRepository
	secrets domain.SecretStore
}

// NewDirectoryConnectorProvisioner builds the provisioner over the tenant
// repository and the secret store (OpenBao in production, the sealed keystore in
// dev).
func NewDirectoryConnectorProvisioner(repo *TenantRepository, secrets domain.SecretStore) *DirectoryConnectorProvisioner {
	return &DirectoryConnectorProvisioner{repo: repo, secrets: secrets}
}

// ConnectorSpec is the configuration of a connector to provision (everything but
// the secret and the vault reference, which the provisioner handles).
type ConnectorSpec struct {
	OrganizationID uuid.UUID
	Kind           domain.DirectoryKind
	Name           string
	ScopeFilter    string
	Attributes     []domain.AttributeMapping
	Groups         []domain.GroupMapping
}

// Provision custodies bindSecret in the vault, builds the connector with the
// resulting reference, and persists it — compensating (deleting the vault entry)
// if construction or persistence fails. bindSecret is used only in memory here and
// is never returned or logged.
func (p *DirectoryConnectorProvisioner) Provision(ctx context.Context, spec ConnectorSpec, bindSecret []byte) (domain.DirectoryConnector, error) {
	// 1) Vault write — remote call, OUTSIDE any DB transaction (RFC-0004 §4).
	ref, err := p.secrets.Put(ctx, bindSecret)
	if err != nil {
		return domain.DirectoryConnector{}, fmt.Errorf("directory_provisioner: custódia da credencial falhou: %w", err)
	}

	// 2) Build the connector with the vault reference (never the secret).
	conn, err := domain.NewDirectoryConnector(spec.OrganizationID, spec.Kind, spec.Name,
		spec.ScopeFilter, ref, spec.Attributes, spec.Groups)
	if err != nil {
		p.compensate(ctx, ref)
		return domain.DirectoryConnector{}, err
	}

	// 3) Persist — on failure, the vault entry is orphaned and must be removed.
	if err := p.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		return NewDirectoryConnectorStore(ttx).Create(ctx, conn)
	}); err != nil {
		p.compensate(ctx, ref)
		return domain.DirectoryConnector{}, err
	}
	return conn, nil
}

// ResolveCredential fetches the connector's bind credential from the vault for the
// syncer. The secret is returned for in-memory use and must never be persisted.
func (p *DirectoryConnectorProvisioner) ResolveCredential(ctx context.Context, conn domain.DirectoryConnector) ([]byte, error) {
	secret, err := p.secrets.Get(ctx, conn.CredentialRef)
	if err != nil {
		return nil, fmt.Errorf("directory_provisioner: resolução da credencial falhou: %w", err)
	}
	return secret, nil
}

// compensate best-effort removes an orphaned vault entry after a failed provision.
func (p *DirectoryConnectorProvisioner) compensate(ctx context.Context, ref string) {
	_ = p.secrets.Delete(ctx, ref)
}
