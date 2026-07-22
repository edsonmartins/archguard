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
)

// The acting principal — who is performing the current operation — travels in
// the context, set by the authenticated request boundary (the HTTP handler /
// session middleware) and read by the business operations that audit
// themselves (T-017). Keeping it in the context, not in every method signature,
// means the operation contracts don't have to thread an actor through — the
// authenticated boundary establishes it once.

type principalKey struct{}

// ErrNoPrincipal is returned when an operation that must record WHO acted finds
// no principal in the context. Fail-closed: an administrative mutation that
// cannot name its actor is not audited-as-anonymous — it is refused.
var ErrNoPrincipal = errors.New("principal: nenhum ator autenticado no contexto")

// WithPrincipal returns a context carrying the acting principal (its audit
// actor: the opaque subject and, when known, the membership and session).
func WithPrincipal(ctx context.Context, actor AuditActor) context.Context {
	return context.WithValue(ctx, principalKey{}, actor)
}

// PrincipalFromContext returns the acting principal, or false if none is set.
func PrincipalFromContext(ctx context.Context) (AuditActor, bool) {
	actor, ok := ctx.Value(principalKey{}).(AuditActor)
	if !ok || actor.IdentitySubject == "" {
		return AuditActor{}, false
	}
	return actor, true
}
