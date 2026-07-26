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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgDisplayNamer resolves organization UUIDs to their display names, for the
// console's tenant selector (a UUID is unusable in the UI). The `organization`
// table is the legacy Casdoor table and carries no RLS (migration 0011 enabled RLS
// only on the tenant-scoped domain tables), so a plain pooled read is correct and
// tenant-safe: it exposes only the display name for ids the caller already holds
// (its own memberships, listed via the authorized global path).
type OrgDisplayNamer struct {
	pool *pgxpool.Pool
}

// NewOrgDisplayNamer builds the namer over the runtime pool.
func NewOrgDisplayNamer(pool *pgxpool.Pool) *OrgDisplayNamer {
	return &OrgDisplayNamer{pool: pool}
}

// DisplayNames returns a map organizationID → display_name for the given ids. Ids
// with no row (or an empty display name) are simply absent from the map; the caller
// falls back to the id. A query failure is returned — never a partial map served as
// complete (the caller decides, but the error is explicit).
func (n *OrgDisplayNamer) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, id.String())
	}
	rows, err := n.pool.Query(ctx,
		`SELECT id::text, display_name FROM organization WHERE id::text = ANY($1)`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var idText, name string
		if err := rows.Scan(&idText, &name); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(idText)
		if err != nil || name == "" {
			continue
		}
		out[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
