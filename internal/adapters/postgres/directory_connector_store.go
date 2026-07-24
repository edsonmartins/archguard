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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrConnectorNotFound is returned when no connector matches within the tenant.
var ErrConnectorNotFound = errors.New("postgres: conector de diretório não encontrado")

// DirectoryConnectorStore is the tenant-scoped store for directory connectors
// (pacote 009, T-001). Built on a TenantTx, every operation is confined to the
// tenant two ways: the explicit organization_id predicate/guard (Barreira 1) and
// the SET LOCAL the TenantTx applied (Barreira 2 / RLS). The connector's secret is
// NEVER here — only its vault reference (INV-7).
type DirectoryConnectorStore struct {
	ttx *TenantTx
}

// NewDirectoryConnectorStore builds the store on an open tenant transaction.
func NewDirectoryConnectorStore(ttx *TenantTx) *DirectoryConnectorStore {
	return &DirectoryConnectorStore{ttx: ttx}
}

// Create inserts a connector. It refuses a row whose organization differs from the
// store's tenant (ErrCrossTenantWrite). The versioned mapping is stored as jsonb.
func (s *DirectoryConnectorStore) Create(ctx context.Context, c domain.DirectoryConnector) error {
	scope := s.ttx.scope.OrganizationID()
	if c.OrganizationID != scope {
		return fmt.Errorf("%w: alvo %s, escopo %s", ErrCrossTenantWrite, c.OrganizationID, scope)
	}
	mapping, err := json.Marshal(c.Mapping)
	if err != nil {
		return fmt.Errorf("postgres: serialização do mapeamento falhou: %w", err)
	}
	const q = `
		INSERT INTO directory_connector
			(id, organization_id, kind, name, scope_filter, credential_ref, enabled,
			 mapping_version, mapping)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := s.ttx.tx.Exec(ctx, q,
		c.ID.String(), c.OrganizationID.String(), string(c.Kind), c.Name,
		c.ScopeFilter, c.CredentialRef, c.Enabled, c.Mapping.Version, mapping); err != nil {
		return fmt.Errorf("postgres: criação de directory_connector falhou: %w", err)
	}
	return nil
}

// Get returns the connector by id WITHIN the store's tenant (Barreira 1: a
// connector of another tenant yields ErrConnectorNotFound).
func (s *DirectoryConnectorStore) Get(ctx context.Context, id uuid.UUID) (domain.DirectoryConnector, error) {
	const q = `
		SELECT id::text, organization_id::text, kind, name, scope_filter, credential_ref,
		       enabled, mapping
		FROM directory_connector
		WHERE id = $1 AND organization_id = $2`
	row := s.ttx.tx.QueryRow(ctx, q, id.String(), s.ttx.scope.OrganizationID().String())
	c, err := scanConnector(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DirectoryConnector{}, ErrConnectorNotFound
	}
	return c, err
}

// List returns the tenant's connectors, ordered by name.
func (s *DirectoryConnectorStore) List(ctx context.Context) ([]domain.DirectoryConnector, error) {
	const q = `
		SELECT id::text, organization_id::text, kind, name, scope_filter, credential_ref,
		       enabled, mapping
		FROM directory_connector
		WHERE organization_id = $1
		ORDER BY name`
	rows, err := s.ttx.tx.Query(ctx, q, s.ttx.scope.OrganizationID().String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de directory_connector falhou: %w", err)
	}
	defer rows.Close()
	var out []domain.DirectoryConnector
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de directory_connector falhou: %w", err)
	}
	return out, nil
}

// scanRow abstracts pgx.Row / pgx.Rows for the shared scan.
type scanRow interface {
	Scan(dest ...any) error
}

func scanConnector(row scanRow) (domain.DirectoryConnector, error) {
	var idText, orgText, kind, name, scope, credRef string
	var enabled bool
	var mappingJSON []byte
	if err := row.Scan(&idText, &orgText, &kind, &name, &scope, &credRef, &enabled, &mappingJSON); err != nil {
		return domain.DirectoryConnector{}, err
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return domain.DirectoryConnector{}, fmt.Errorf("postgres: id inválido %q: %w", idText, err)
	}
	org, err := uuid.Parse(orgText)
	if err != nil {
		return domain.DirectoryConnector{}, fmt.Errorf("postgres: organization_id inválido %q: %w", orgText, err)
	}
	var mapping domain.ConnectorMapping
	if err := json.Unmarshal(mappingJSON, &mapping); err != nil {
		return domain.DirectoryConnector{}, fmt.Errorf("postgres: mapeamento inválido: %w", err)
	}
	return domain.DirectoryConnector{
		ID:             id,
		OrganizationID: org,
		Kind:           domain.DirectoryKind(kind),
		Name:           name,
		ScopeFilter:    scope,
		CredentialRef:  credRef,
		Enabled:        enabled,
		Mapping:        mapping,
	}, nil
}
