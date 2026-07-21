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

// ErrMalformedCredential is returned by Create when the credential does not carry
// exactly the material its type allows (domain.Credential.WellFormed) — the
// application-side guard for INV-7, ahead of the database credential_shape CHECK.
var ErrMalformedCredential = errors.New("postgres: credencial malformada (INV-7)")

// CredentialStore persists identity credentials (RFC-0002 §2.4). Credentials are
// CROSS-TENANT (they belong to the global identity), so the store carries no
// tenant context.
type CredentialStore struct {
	db Querier
}

// NewCredentialStore builds a store over any Querier (pool or transaction).
func NewCredentialStore(db Querier) *CredentialStore {
	return &CredentialStore{db: db}
}

// Create inserts a credential. It refuses a malformed credential before touching
// the database (ErrMalformedCredential): a store must never be the place a
// reversible secret slips into the clear. The database credential_shape CHECK is
// the second barrier.
func (s *CredentialStore) Create(ctx context.Context, c domain.Credential) error {
	if !c.WellFormed() {
		return ErrMalformedCredential
	}
	params, err := marshalParams(c.Params)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO credential
			(id, identity_id, type, aal, verifier, secret_ref, public_material, params, label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err = s.db.Exec(ctx, q,
		c.ID.String(), c.IdentityID.String(), string(c.Type), string(c.AAL),
		nullBytes(c.Verifier), nullString(c.SecretRef), nullBytes(c.PublicMaterial),
		params, nullString(c.Label))
	if err != nil {
		return fmt.Errorf("postgres: criação de credencial falhou: %w", err)
	}
	return nil
}

// ListByIdentity returns all credentials of an identity, ordered by creation.
func (s *CredentialStore) ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]domain.Credential, error) {
	const q = `
		SELECT id::text, identity_id::text, type, aal,
		       verifier, secret_ref, public_material, params, label
		FROM credential
		WHERE identity_id = $1
		ORDER BY created_at`
	rows, err := s.db.Query(ctx, q, identityID.String())
	if err != nil {
		return nil, fmt.Errorf("postgres: listagem de credenciais falhou: %w", err)
	}
	defer rows.Close()

	var out []domain.Credential
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iteração de credenciais falhou: %w", err)
	}
	return out, nil
}

func scanCredential(row pgx.Row) (domain.Credential, error) {
	var (
		c          domain.Credential
		idText     string
		idnText    string
		typ        string
		aal        string
		secretRef  *string
		label      *string
		paramsJSON []byte
	)
	if err := row.Scan(&idText, &idnText, &typ, &aal,
		&c.Verifier, &secretRef, &c.PublicMaterial, &paramsJSON, &label); err != nil {
		return domain.Credential{}, fmt.Errorf("postgres: leitura de credencial falhou: %w", err)
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("postgres: id de credencial inválido %q: %w", idText, err)
	}
	idn, err := uuid.Parse(idnText)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("postgres: identity_id inválido %q: %w", idnText, err)
	}
	c.ID = id
	c.IdentityID = idn
	c.Type = domain.FactorType(typ)
	c.AAL = domain.AAL(aal)
	if secretRef != nil {
		c.SecretRef = *secretRef
	}
	if label != nil {
		c.Label = *label
	}
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &c.Params); err != nil {
			return domain.Credential{}, fmt.Errorf("postgres: params de credencial ilegíveis: %w", err)
		}
	}
	return c, nil
}

// marshalParams renders the params map as JSON, using an empty object for a nil
// or empty map (never SQL NULL — the column is NOT NULL DEFAULT '{}').
func marshalParams(p map[string]string) ([]byte, error) {
	if len(p) == 0 {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("postgres: params de credencial não serializáveis: %w", err)
	}
	return b, nil
}

// nullBytes maps an empty slice to a SQL NULL so the credential_shape CHECK sees
// an absent column rather than an empty value.
func nullBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

// nullString maps an empty string to a SQL NULL for the same reason.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
