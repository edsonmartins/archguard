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
	"errors"
	"fmt"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrIdentityNotFound is returned by the identity lookups when no row matches.
var ErrIdentityNotFound = errors.New("postgres: identidade não encontrada")

// Querier is the narrow slice of pgx used by the stores: it is satisfied by both
// *pgxpool.Pool (outside a transaction) and pgx.Tx (inside WithTx), so a store
// works in either context without knowing which (RFC-0002 §5: one transaction
// per business operation).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// IdentityStore persists the global identity (RFC-0002 §2.1). The identity is
// CROSS-TENANT by construction — it carries no organization_id and is not scoped
// to a tenant — so this store needs no tenant context (unlike the tenant-scoped
// repositories of T-008). UUIDs cross the boundary as text (id::text / String())
// so the store does not depend on a pgx UUID codec being registered.
type IdentityStore struct {
	db Querier
}

// NewIdentityStore builds a store over any Querier (pool or transaction).
func NewIdentityStore(db Querier) *IdentityStore {
	return &IdentityStore{db: db}
}

// Create inserts a new identity. created_at/updated_at are assigned by the
// database defaults; this method does not mutate idn, so a caller that needs the
// stamped timestamps re-reads the row. A duplicate subject or email_hash surfaces
// as the underlying unique-constraint error.
func (s *IdentityStore) Create(ctx context.Context, idn domain.Identity) error {
	const q = `
		INSERT INTO identity
			(id, subject, type, status, primary_email_enc, email_hash, display_name_enc)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.Exec(ctx, q,
		idn.ID.String(), idn.Subject, string(idn.Type), string(idn.Status),
		idn.PrimaryEmailEnc, idn.EmailHash, idn.DisplayNameEnc)
	if err != nil {
		return fmt.Errorf("postgres: criação de identidade falhou: %w", err)
	}
	return nil
}

// FindByEmailHash returns the identity whose email_hash matches, or
// ErrIdentityNotFound. The lookup rides the partial unique index (migration
// 0005). A nil/empty hash is rejected: an empty hash is not a lookup key.
func (s *IdentityStore) FindByEmailHash(ctx context.Context, emailHash []byte) (domain.Identity, error) {
	if len(emailHash) == 0 {
		return domain.Identity{}, ErrIdentityNotFound
	}
	const q = `
		SELECT id::text, subject, type, status,
		       primary_email_enc, email_hash, display_name_enc,
		       created_at, updated_at
		FROM identity
		WHERE email_hash = $1`
	return scanIdentity(s.db.QueryRow(ctx, q, emailHash))
}

// FindByEmail hashes the e-mail through the custodian and looks the identity up
// by email_hash — the login-by-hash path (RFC-0002 §2.1): plaintext is never
// stored or compared, only its deployment-keyed HMAC. An e-mail that normalizes
// to empty yields domain.ErrEmptyEmail from the custodian.
func (s *IdentityStore) FindByEmail(ctx context.Context, custodian domain.KeyCustodian, email string) (domain.Identity, error) {
	hash, err := custodian.HashEmail(email)
	if err != nil {
		return domain.Identity{}, err
	}
	return s.FindByEmailHash(ctx, hash)
}

// scanIdentity maps one row into a domain.Identity, translating no-rows into
// ErrIdentityNotFound and parsing the text-encoded UUID.
func scanIdentity(row pgx.Row) (domain.Identity, error) {
	var (
		idn    domain.Identity
		idText string
		typ    string
		status string
	)
	err := row.Scan(&idText, &idn.Subject, &typ, &status,
		&idn.PrimaryEmailEnc, &idn.EmailHash, &idn.DisplayNameEnc,
		&idn.CreatedAt, &idn.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Identity{}, ErrIdentityNotFound
	}
	if err != nil {
		return domain.Identity{}, fmt.Errorf("postgres: leitura de identidade falhou: %w", err)
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return domain.Identity{}, fmt.Errorf("postgres: id de identidade inválido %q: %w", idText, err)
	}
	idn.ID = id
	idn.Type = domain.IdentityType(typ)
	idn.Status = domain.IdentityStatus(status)
	return idn, nil
}
