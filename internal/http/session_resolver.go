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

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// LegacyBinding reads the ArchGuard identity id and auth_session id that the login
// flow BOUND into the legacy (Beego) session on a successful login. It is the
// ONLY legacy-session-aware seam of the resolver; a thin Beego adapter at the app
// boundary implements it (reading two values from the framework session). A
// missing binding means "not logged in" (ok=false), not an error.
type LegacyBinding interface {
	Bound(r *http.Request) (identityID, sessionID uuid.UUID, ok bool)
}

// SessionLoader loads a persisted auth_session by identity and id — the
// postgres.SessionBridge implements it.
type SessionLoader interface {
	LoadSession(ctx context.Context, identityID, sessionID uuid.UUID) (domain.AuthSession, error)
}

// BridgingResolver implements SessionResolver by translating a legacy login into
// the new-model auth_session: it reads the identity+session binding the login
// placed in the legacy session, then loads that auth_session. The heavy lifting
// (establishing the session from login facts) happens ONCE at login time
// (SessionBridge.EstablishSession); this hot path is a single keyed load. A
// binding to a revoked/absent session resolves to "not logged in", so a stale
// binding never grants access.
type BridgingResolver struct {
	binding LegacyBinding
	loader  SessionLoader
}

// NewBridgingResolver builds the resolver over the legacy binding and the session
// loader.
func NewBridgingResolver(binding LegacyBinding, loader SessionLoader) *BridgingResolver {
	return &BridgingResolver{binding: binding, loader: loader}
}

// Session resolves the request's authenticated session, or (nil, false) when
// there is no valid binding or the bound session no longer loads.
func (r *BridgingResolver) Session(req *http.Request) (*domain.AuthSession, bool) {
	identityID, sessionID, ok := r.binding.Bound(req)
	if !ok {
		return nil, false
	}
	session, err := r.loader.LoadSession(req.Context(), identityID, sessionID)
	if err != nil {
		return nil, false
	}
	// A revoked session is not a valid login context.
	if session.Status != domain.SessionActive {
		return nil, false
	}
	return &session, true
}

// ensure it satisfies the SessionResolver port.
var _ SessionResolver = (*BridgingResolver)(nil)
