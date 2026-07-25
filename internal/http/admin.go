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

package http

import (
	"context"
	"net/http"
)

// adminCtxKey is the private context key under which the Beego bridge records
// whether the caller is an administrator.
type adminCtxKey struct{}

// WithAdmin returns a context flagging the caller's administrator status. The
// bridge derives it from the legacy admin concept (IsAdmin / IsGlobalAdmin) — the
// authorization for console-CRUD operations, which the pacote 007 PDP (asset/PAM
// scoped) does not model. Absent ⇒ not an administrator (fail-closed).
func WithAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminCtxKey{}, isAdmin)
}

// AdminFromContext reports whether the bridge marked the caller an administrator.
func AdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(adminCtxKey{}).(bool)
	return v
}

// RequireAdmin wraps a handler so it runs only for an administrator caller, else
// 403. It is the console-CRUD authorization gate, layered UNDER the assurance
// pipeline: assurance (INV-8) and tenant (INV-5) are enforced first, then this
// admin gate. Asset/PAM operations are authorized by the PDP instead, not here.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !AdminFromContext(r.Context()) {
			writeError(w, http.StatusForbidden, "operação restrita a administradores")
			return
		}
		next.ServeHTTP(w, r)
	})
}
