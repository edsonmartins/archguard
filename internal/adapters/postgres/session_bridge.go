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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionBridge establishes and loads new-model auth_session rows for an
// identity — the core of connecting an authenticated login to the OIDC layer
// (the SessionResolver of the /authorize endpoint). It resolves the identity's
// memberships through the authorized-and-audited GlobalRepository (the login-time
// cross-tenant read, RFC-0002 §4), and persists the session under the identity
// scope. It is deliberately free of any legacy-session detail — a Beego-aware
// resolver supplies the identity and the proven login facts.
type SessionBridge struct {
	pool   *pgxpool.Pool
	global *GlobalRepository
}

// NewSessionBridge builds the bridge over the pool and the global repository
// (whose authorizer/auditor gate the cross-tenant membership read).
func NewSessionBridge(pool *pgxpool.Pool, global *GlobalRepository) *SessionBridge {
	return &SessionBridge{pool: pool, global: global}
}

// EstablishSession creates and persists an active (or pending, if the identity
// has several active memberships) auth_session for the identity, stamped with the
// proven assurance and methods of the login. It returns the persisted session,
// whose ID a resolver stores to bind the legacy session to this one. A denial —
// no active membership — surfaces as domain.ErrNoActiveMembership.
func (b *SessionBridge) EstablishSession(ctx context.Context, identity domain.Identity, provenAAL domain.AAL, methods []domain.FactorType, now time.Time) (domain.AuthSession, error) {
	var memberships []domain.Membership
	err := b.global.WithGlobalTx(ctx, domain.GlobalAccess{
		Principal: identity.Subject,
		Reason:    "login: resolução de memberships da própria identidade",
		Scope:     domain.ScopeSelf, // confinado à própria identidade (ADR-0022)
	}, func(tx pgx.Tx) error {
		var e error
		memberships, e = NewMembershipStore(tx).ListByIdentity(ctx, identity.ID)
		return e
	})
	if err != nil {
		return domain.AuthSession{}, err
	}

	session, err := domain.NewAuthSession(identity.ID, provenAAL, memberships)
	if err != nil {
		return domain.AuthSession{}, err // ErrNoActiveMembership on a person with no live tenant
	}
	if err := session.SetAuthContext(now, methods); err != nil {
		return domain.AuthSession{}, err
	}

	scope, err := domain.NewIdentityScope(identity.ID)
	if err != nil {
		return domain.AuthSession{}, err
	}
	if err := NewIdentityRepository(b.pool, scope).WithIdentityTx(ctx, func(itx *IdentityTx) error {
		return NewIdentitySessionStore(itx).Create(ctx, session)
	}); err != nil {
		return domain.AuthSession{}, err
	}
	return session, nil
}

// LoadSession loads a previously established session by id, confined to the
// identity (a session of another identity yields ErrSessionNotFound). This is the
// hot path a resolver runs on every request once the session is bound.
func (b *SessionBridge) LoadSession(ctx context.Context, identityID, sessionID uuid.UUID) (domain.AuthSession, error) {
	scope, err := domain.NewIdentityScope(identityID)
	if err != nil {
		return domain.AuthSession{}, err
	}
	var out domain.AuthSession
	err = NewIdentityRepository(b.pool, scope).WithIdentityTx(ctx, func(itx *IdentityTx) error {
		var e error
		out, e = NewIdentitySessionStore(itx).Get(ctx, sessionID)
		return e
	})
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("session_bridge: carga de sessão falhou: %w", err)
	}
	return out, nil
}
