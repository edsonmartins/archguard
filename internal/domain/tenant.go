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

package domain

import (
	"errors"

	"github.com/google/uuid"
)

// RLSOrgSettingName is the PostgreSQL session setting that carries the active
// tenant. The application sets it per transaction (SET LOCAL) and the Row-Level
// Security policies (T-010) read it via current_setting(). Kept here so the
// application side has one source of truth; the RLS migration must use the SAME
// literal string.
const RLSOrgSettingName = "app.current_organization"

// RLSGlobalReadSettingName is the PostgreSQL session setting the GlobalRepository
// sets ('on') to perform an authorized, audited cross-tenant read. The RLS
// policies (T-010) permit a row when it matches the active tenant OR this flag is
// on. Set LOCAL, per transaction, only after authorization and audit succeed.
const RLSGlobalReadSettingName = "app.global_read"

// RLSIdentitySettingName is the PostgreSQL session setting that carries the
// identity context of the login flow (T-012). auth_session rows pending tenant
// selection have no organization, so the tenant setting cannot scope them; the
// IdentityRepository sets this per transaction (SET LOCAL) and the auth_session
// RLS policy (migration 0013) reads it. Same one-source-of-truth rule as the
// other setting names: the migration must use this SAME literal string.
const RLSIdentitySettingName = "app.current_identity"

// ErrNoTenant is returned when a tenant-scoped repository is asked to operate
// without a tenant. Barreira 1 (RFC-0002 §4): there is no data access without a
// tenant context.
var ErrNoTenant = errors.New("tenant: contexto de tenant é obrigatório")

// TenantScope is the mandatory tenant context of a tenant-scoped repository. It
// is the application half of the two-barrier isolation (RFC-0002 §4): a
// repository cannot be built without one, so no query runs unscoped. A
// cross-tenant read (global reports) uses a distinct, explicit, audited type
// (GlobalRepository, T-009) — never a TenantScope with a wildcard.
type TenantScope struct {
	organizationID uuid.UUID
}

// NewTenantScope builds a scope for organizationID. It refuses uuid.Nil: an empty
// tenant is not a valid scope, it is the absence of one (ErrNoTenant).
func NewTenantScope(organizationID uuid.UUID) (TenantScope, error) {
	if organizationID == uuid.Nil {
		return TenantScope{}, ErrNoTenant
	}
	return TenantScope{organizationID: organizationID}, nil
}

// OrganizationID returns the tenant this scope is bound to.
func (s TenantScope) OrganizationID() uuid.UUID {
	return s.organizationID
}
