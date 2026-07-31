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
	"errors"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// BridgeLogin establishes a new-model auth_session for a just-authenticated user
// and returns the identity + session ids to bind into the framework session, so
// the control-plane API can resolve the session (T-004).
//
// It is RESOLVE-ONLY (the seed provisions identities, never the login path): an
// identity not yet in the new model, or a profile without custody (conformant),
// yields established=false with NO error — the legacy login proceeds and the
// domain API simply stays fail-closed (401) for that user. A genuine failure
// (e.g. the identity has no active membership) is returned as an error for the
// caller to LOG; it must not fail the legacy login, which already succeeded.
//
// methods are the factors actually proven by this login; provenAAL is derived
// from them and never exceeds their ceiling (SetAuthContext enforces this).
func BridgeLogin(ctx context.Context, email string, methods []domain.FactorType, now time.Time) (identityID, sessionID uuid.UUID, established bool, err error) {
	f := ActiveFactory()
	if f == nil {
		return uuid.Nil, uuid.Nil, false, nil
	}
	custodian, cerr := f.KeyCustodian()
	if cerr != nil {
		// Custody unavailable (conformant, OpenBao not wired) — no domain session.
		return uuid.Nil, uuid.Nil, false, nil
	}

	pool := f.Pool()
	idn, ferr := postgres.NewIdentityStore(pool).FindByEmail(ctx, custodian, email)
	if errors.Is(ferr, postgres.ErrIdentityNotFound) {
		// Not provisioned in the new model — resolve-only, never create at login.
		return uuid.Nil, uuid.Nil, false, nil
	}
	if ferr != nil {
		return uuid.Nil, uuid.Nil, false, ferr
	}

	global := newGlobalRepository(pool)
	session, serr := postgres.NewSessionBridge(pool, global).EstablishSession(ctx, idn, provenAALFromMethods(methods), methods, now)
	if serr != nil {
		return uuid.Nil, uuid.Nil, false, serr
	}
	return idn.ID, session.ID, true, nil
}

// provenAALFromMethods returns the strongest assurance the given factors can
// prove — the highest MaxAAL among them (password→AAL1, TOTP→AAL2, WebAuthn→AAL3).
// It never over-claims: an empty set yields AAL1, and SetAuthContext further caps
// the session at the factors' ceiling.
func provenAALFromMethods(methods []domain.FactorType) domain.AAL {
	best := domain.AAL1
	for _, m := range methods {
		if a := domain.MaxAAL(m); a.AtLeast(best) {
			best = a
		}
	}
	return best
}
