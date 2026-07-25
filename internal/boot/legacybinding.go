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

package boot

import (
	"context"
	"net/http"

	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
)

// Session-value keys the login bridge (T-004b) writes into the framework session
// on a successful login, and the API bridge reads to rebuild the request context.
// They are the contract between the two halves of the seam. Distinct from the
// Beego cookie name ("archguard_session_id") — these are values inside the session.
const (
	SessionKeyIdentityID  = "ag_identity_id"
	SessionKeyAuthSession = "ag_auth_session_id"
)

// bindingKey is the private context key under which the identity+session binding
// travels from the Beego bridge to the LegacyBinding adapter.
type bindingKey struct{}

type sessionBinding struct {
	identityID uuid.UUID
	sessionID  uuid.UUID
}

// WithSessionBinding returns a context carrying the ArchGuard identity and
// auth_session ids that the login flow bound into the framework session. The
// Beego bridge (T-004b) injects it after reading the session; the contextBinding
// adapter reads it inside the net/http pipeline. An absent binding means "not
// logged in" (fail-closed), never an error.
func WithSessionBinding(ctx context.Context, identityID, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, bindingKey{}, sessionBinding{identityID: identityID, sessionID: sessionID})
}

// contextBinding implements apihttp.LegacyBinding by reading the binding the
// bridge placed in the request context. It is the production LegacyBinding: the
// Beego-aware part (reading the framework session) happens in the controller; this
// side only reads the context, keeping the net/http pipeline framework-free.
type contextBinding struct{}

// Bound returns the bound identity and session ids, or ok=false when no binding is
// present — the resolver then treats the request as unauthenticated (fail-closed).
func (contextBinding) Bound(r *http.Request) (identityID, sessionID uuid.UUID, ok bool) {
	b, ok := r.Context().Value(bindingKey{}).(sessionBinding)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return b.identityID, b.sessionID, true
}

var _ apihttp.LegacyBinding = contextBinding{}
