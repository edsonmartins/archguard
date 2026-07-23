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
	"time"
)

// RecommendedIntrospectionTTL is the short cache lifetime a component WITHOUT
// back-channel logout support should honor for an introspection result (RFC-0006
// §6): the shorter the cache, the faster a revocation propagates. It is the
// compensation for the missing logout channel, documented as a limitation — the
// central contract is never degraded to match a weaker component.
const RecommendedIntrospectionTTL = 30 * time.Second

// IntrospectionResponse is the OAuth 2.0 Token Introspection response (RFC 7662).
// For an INACTIVE token only `active: false` is returned — no claim is leaked for
// a token that is revoked, expired or unknown (RFC 7662 §2.2). For an active
// token the non-personal contract claims are echoed so the component can enforce
// them.
type IntrospectionResponse struct {
	Active       bool   `json:"active"`
	Issuer       string `json:"iss,omitempty"`
	Subject      string `json:"sub,omitempty"`
	Audience     string `json:"aud,omitempty"`
	Organization string `json:"org,omitempty"`
	ACR          string `json:"acr,omitempty"`
	SessionID    string `json:"sid,omitempty"`
	ExpiresAt    int64  `json:"exp,omitempty"`
	IssuedAt     int64  `json:"iat,omitempty"`
	PCID         string `json:"pcid,omitempty"`
}

// SessionLiveness reports whether the session behind a token is still active — a
// revoked session (logout, membership revoke, grant expiry) makes its tokens
// introspect as inactive BEFORE they expire, which is how revocation reaches a
// component that has no back-channel logout (RFC-0006 §6). Fail-closed: an
// implementation that cannot determine liveness reports NOT live (the caller
// returns active:false), so an unverifiable token is never treated as active.
type SessionLiveness interface {
	// Live reports whether the session sid is active in organization org.
	Live(ctx context.Context, organizationID, sid string) (bool, error)
}

// BuildIntrospection produces the introspection response for a token whose claims
// were verified: the token is ACTIVE only if its session is still live AND it has
// not expired at now. An inactive token yields ONLY active:false (no claims). The
// short introspection cache (RecommendedIntrospectionTTL) is what makes a
// revocation take effect quickly on a no-logout component.
func BuildIntrospection(claims OIDCClaims, sessionLive bool, now time.Time) IntrospectionResponse {
	active := sessionLive && now.Unix() < claims.ExpiresAt
	if !active {
		return IntrospectionResponse{Active: false}
	}
	return IntrospectionResponse{
		Active:       true,
		Issuer:       claims.Issuer,
		Subject:      claims.Subject,
		Audience:     claims.Audience,
		Organization: claims.Organization,
		ACR:          claims.ACR,
		SessionID:    claims.SessionID,
		ExpiresAt:    claims.ExpiresAt,
		IssuedAt:     claims.IssuedAt,
		PCID:         claims.PCID,
	}
}
