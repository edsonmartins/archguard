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
	"context"
	"errors"
	"fmt"
)

// GlobalAccessScope declares how far a cross-tenant access reaches (ADR-0022).
type GlobalAccessScope int

const (
	// ScopeCrossTenant is a broad read across tenants (global reports, an admin
	// reading another principal's data). It is the DEFAULT — the conservative one,
	// so a call-site that forgets to declare its scope falls into the restricted
	// path, never the permissive one.
	ScopeCrossTenant GlobalAccessScope = iota
	// ScopeSelf is a read confined to the principal's OWN identity (login/console
	// resolving one's own memberships). Intrinsic to authentication; the INV-1
	// guarantee lives in the call-site, which reads only the authenticated identity.
	ScopeSelf
)

// String is the persistable/loggable form of the scope. It matches the CHECK
// constraint of global_access_audit ('self' | 'cross_tenant'); an unknown value
// (should never occur) renders conservatively as cross_tenant.
func (s GlobalAccessScope) String() string {
	if s == ScopeSelf {
		return "self"
	}
	return "cross_tenant"
}

// GlobalAccess describes one cross-tenant access. Cross-tenant reads (global
// reports, "all my memberships") are legitimate but must never be casual: each
// carries a principal (who), a reason (why) and a scope (how far), which the
// authorizer weighs and the auditor records (RFC-0002 §4, ADR-0022).
type GlobalAccess struct {
	// Principal identifies who is performing the cross-tenant access (a subject,
	// an operator, a service). Required.
	Principal string
	// Reason is the declared purpose of the access. Required — a cross-tenant
	// read with no stated reason is refused.
	Reason string
	// Scope declares the confinement of the access (ADR-0022). The zero value is
	// ScopeCrossTenant (the restricted default), so self-confined reads must opt in.
	Scope GlobalAccessScope
}

// Validate refuses an access with no principal or no reason.
func (a GlobalAccess) Validate() error {
	if a.Principal == "" || a.Reason == "" {
		return fmt.Errorf("%w: principal e motivo são obrigatórios", ErrGlobalAccessDenied)
	}
	return nil
}

// Errors for the global-access path.
var (
	// ErrGlobalAccessDenied is returned when a cross-tenant access is refused —
	// by policy, by a missing principal/reason, or (fail-closed, INV-6) because
	// the authorizer could not reach a decision.
	ErrGlobalAccessDenied = errors.New("global: acesso cross-tenant negado")
	// ErrGlobalAuditUnavailable is returned when the access could not be recorded
	// durably. By I-5.4 the operation is then denied — a cross-tenant read that
	// cannot be audited does not happen.
	ErrGlobalAuditUnavailable = errors.New("global: auditoria indisponível — operação negada")
)

// GlobalAuthorizer decides whether a cross-tenant access may proceed. Fine-grained
// authorization is OpenFGA's job (I-7.4, pacote 007); this port lets the global
// repository ask before crossing tenants. INV-6: an authorizer that cannot decide
// MUST return an error (deny), never allow.
type GlobalAuthorizer interface {
	Authorize(ctx context.Context, access GlobalAccess) error
}

// AccessAuditor records a cross-tenant access durably before it happens. The
// tamper-evident append-only trail is pacote 003; this port is the seam. I-5.4:
// if Record fails, the caller denies the operation (fail-closed).
type AccessAuditor interface {
	Record(ctx context.Context, access GlobalAccess) error
}
