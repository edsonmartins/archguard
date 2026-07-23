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
	"time"

	"github.com/google/uuid"
)

// BackchannelLogoutEvent is the OIDC back-channel logout event member (OpenID
// Connect Back-Channel Logout 1.0 §2.4): its presence in the token's `events`
// marks it as a logout token, distinguishing it from an id/access token.
const BackchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

// LogoutTokenClaims is the OIDC back-channel logout token (RFC-0006 §6). It
// carries the session id (`sid`) of the session being ended and the logout event;
// a component receiving it terminates the derived session bound to that sid. No
// personal data — sid and sub are opaque.
type LogoutTokenClaims struct {
	Issuer        string                            `json:"iss"`
	Audience      string                            `json:"aud"`
	IssuedAt      int64                             `json:"iat"`
	JTI           string                            `json:"jti"`
	SID           string                            `json:"sid"`
	Events        map[string]map[string]interface{} `json:"events"`
	ClaimsVersion string                            `json:"archguard_claims_version"`
}

// NewLogoutTokenClaims builds a back-channel logout token for one component's
// audience and the ended session's sid. jti is a unique token id (replay
// protection). The events member is set to exactly the back-channel logout event
// with an empty object, per the OIDC spec.
func NewLogoutTokenClaims(issuer, audience, sid, jti string, iat time.Time) (LogoutTokenClaims, error) {
	if issuer == "" || audience == "" || sid == "" || jti == "" {
		return LogoutTokenClaims{}, fmt.Errorf("%w: iss/aud/sid/jti obrigatórios", ErrInvalidClaims)
	}
	if iat.IsZero() {
		return LogoutTokenClaims{}, fmt.Errorf("%w: iat ausente", ErrInvalidClaims)
	}
	return LogoutTokenClaims{
		Issuer:        issuer,
		Audience:      audience,
		IssuedAt:      iat.Unix(),
		JTI:           jti,
		SID:           sid,
		Events:        map[string]map[string]interface{}{BackchannelLogoutEvent: {}},
		ClaimsVersion: OIDCClaimsVersion,
	}, nil
}

// WellFormed reports whether the logout token carries the required claims AND the
// back-channel logout event — a logout token WITHOUT the event is not a logout
// token and must never be minted (it could otherwise be mistaken for an id
// token, OIDC BCL §2.4 security consideration).
func (c LogoutTokenClaims) WellFormed() error {
	switch {
	case c.Issuer == "":
		return fmt.Errorf("%w: iss ausente", ErrInvalidClaims)
	case c.Audience == "":
		return fmt.Errorf("%w: aud ausente", ErrInvalidClaims)
	case c.SID == "":
		return fmt.Errorf("%w: sid ausente", ErrInvalidClaims)
	case c.JTI == "":
		return fmt.Errorf("%w: jti ausente", ErrInvalidClaims)
	case c.IssuedAt <= 0:
		return fmt.Errorf("%w: iat ausente", ErrInvalidClaims)
	}
	if _, ok := c.Events[BackchannelLogoutEvent]; !ok {
		return fmt.Errorf("%w: evento de back-channel logout ausente", ErrInvalidClaims)
	}
	return nil
}

// LogoutNotifier delivers a signed back-channel logout token to a component's
// backchannel_logout_uri (RFC-0006 §6). It is the seam for the HTTP POST; the
// provisional dev implementation records deliveries. A component without
// back-channel support is handled by short-TTL introspection instead (T-010),
// documented as a limitation — not by degrading this contract.
type LogoutNotifier interface {
	// SendLogout POSTs the signed logout token to endpoint. An error means the
	// component was not reached; the caller records the failure but the LOCAL
	// revocation (session + tokens) has already happened regardless.
	SendLogout(ctx context.Context, endpoint, logoutTokenJWT string) error
}

// SessionRevoker revokes a session AND its derived tokens locally — the
// fail-closed leg of logout (spec "as sessões derivadas são encerradas"). The
// postgres implementation composes the auth_session revoke with the refresh-token
// family revocation (RevokeBySession).
type SessionRevoker interface {
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
}

// ErrLocalRevocationFailed wraps a failure to revoke the session/tokens locally
// during logout — the ONLY logout failure that is fatal (a failed component
// notification is compensated by introspection). Callers gate on it with
// errors.Is to decide whether the logout truly failed.
var ErrLocalRevocationFailed = errors.New("logout: revogação local falhou")

// LogoutSigner mints the signed back-channel logout token — the oidc.Signer
// implements it.
type LogoutSigner interface {
	SignLogoutToken(claims LogoutTokenClaims) (string, error)
}

// LogoutClient is a component to notify on logout: its token audience and its
// backchannel_logout_uri.
type LogoutClient struct {
	Audience string
	Endpoint string
}

// LogoutPropagator ends a session at ArchGuard and propagates the back-channel
// logout to the components (RFC-0006 §6). Local revocation is FAIL-CLOSED: if the
// session and its derived tokens cannot be revoked, the logout does not proceed
// and nothing is sent. Component notifications are best-effort — a component that
// cannot be reached keeps its session only until short-TTL introspection catches
// up (T-010); the propagator RETURNS which sends failed so the caller does not
// pretend a partial logout is complete.
type LogoutPropagator struct {
	issuer   string
	signer   LogoutSigner
	notifier LogoutNotifier
	revoker  SessionRevoker
}

// NewLogoutPropagator builds the propagator.
func NewLogoutPropagator(issuer string, signer LogoutSigner, notifier LogoutNotifier, revoker SessionRevoker) *LogoutPropagator {
	return &LogoutPropagator{issuer: issuer, signer: signer, notifier: notifier, revoker: revoker}
}

// Logout revokes the session (and derived tokens) locally, then sends a signed
// back-channel logout token to each component that holds a session for this sid.
// It returns a non-nil error if the LOCAL revocation failed (fail-closed) OR if
// any component notification failed (the errors are joined), so a caller can tell
// a fully-propagated logout from one that must lean on introspection.
func (p *LogoutPropagator) Logout(ctx context.Context, sessionID uuid.UUID, sid string, clients []LogoutClient, iat time.Time) error {
	// Fail-closed local revocation first: if we cannot end the derived sessions,
	// we do not send logout tokens that would falsely imply completion.
	if err := p.revoker.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: %v", ErrLocalRevocationFailed, err)
	}
	var sendErrs []error
	for _, c := range clients {
		claims, err := NewLogoutTokenClaims(p.issuer, c.Audience, sid, uuid.NewString(), iat)
		if err != nil {
			sendErrs = append(sendErrs, fmt.Errorf("logout token para %s: %w", c.Audience, err))
			continue
		}
		token, err := p.signer.SignLogoutToken(claims)
		if err != nil {
			sendErrs = append(sendErrs, fmt.Errorf("assinatura para %s: %w", c.Audience, err))
			continue
		}
		if err := p.notifier.SendLogout(ctx, c.Endpoint, token); err != nil {
			sendErrs = append(sendErrs, fmt.Errorf("entrega a %s: %w", c.Audience, err))
		}
	}
	return errors.Join(sendErrs...)
}

// EndSessionService is the RP-initiated logout composition (the /logout endpoint
// depends on it): it propagates back-channel logout to the registered components
// that support it and revokes the session locally. It returns an error ONLY when
// the LOCAL revocation failed (ErrLocalRevocationFailed) — a failed component
// notification is not fatal to the user's logout (introspection compensates), so
// it is swallowed here (the propagator still records it).
type EndSessionService struct {
	propagator *LogoutPropagator
	registry   *ClientRegistry
	now        func() time.Time
}

// NewEndSessionService builds the service. now supplies the logout token's iat.
func NewEndSessionService(propagator *LogoutPropagator, registry *ClientRegistry, now func() time.Time) *EndSessionService {
	if now == nil {
		now = time.Now
	}
	return &EndSessionService{propagator: propagator, registry: registry, now: now}
}

// EndSession revokes the session and sends back-channel logout to every
// registered component that supports it.
func (s *EndSessionService) EndSession(ctx context.Context, sessionID uuid.UUID, sid string) error {
	var clients []LogoutClient
	for _, id := range s.registry.IDs() {
		c, err := s.registry.Lookup(id)
		if err == nil && c.SupportsBackchannelLogout() {
			clients = append(clients, LogoutClient{Audience: c.Audience, Endpoint: c.BackchannelLogoutURI})
		}
	}
	err := s.propagator.Logout(ctx, sessionID, sid, clients, s.now())
	if errors.Is(err, ErrLocalRevocationFailed) {
		return err // the only fatal case
	}
	return nil // failed component sends are non-fatal to the user's logout
}
