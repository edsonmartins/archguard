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

import "fmt"

// OIDCEndpoints are the absolute URLs of the ArchGuard authorization server's
// endpoints, published in the discovery document.
type OIDCEndpoints struct {
	Authorization string
	Token         string
	JWKS          string
	Introspection string
	EndSession    string
}

// DiscoveryDocument is the OpenID Connect discovery metadata
// (/.well-known/openid-configuration). It is BUILT from the actual flow policy —
// it advertises only "code" response type, S256 PKCE and no implicit/ROPC — so
// the document can never claim to support a flow the server refuses (T-005). It
// carries no personal data.
type DiscoveryDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	BackchannelLogoutSupported        bool     `json:"backchannel_logout_supported"`
	BackchannelLogoutSessionSupported bool     `json:"backchannel_logout_session_supported"`
}

// BuildDiscoveryDocument assembles the discovery metadata from the issuer and the
// endpoint URLs, reflecting the flow policy (only Authorization Code + PKCE S256
// and the device grant; no implicit/ROPC). It refuses an empty issuer or endpoint.
func BuildDiscoveryDocument(issuer string, ep OIDCEndpoints) (DiscoveryDocument, error) {
	if issuer == "" {
		return DiscoveryDocument{}, fmt.Errorf("%w: issuer ausente", ErrInvalidClaims)
	}
	if ep.Authorization == "" || ep.Token == "" || ep.JWKS == "" || ep.Introspection == "" || ep.EndSession == "" {
		return DiscoveryDocument{}, fmt.Errorf("%w: endpoints incompletos", ErrInvalidClaims)
	}
	return DiscoveryDocument{
		Issuer:                issuer,
		AuthorizationEndpoint: ep.Authorization,
		TokenEndpoint:         ep.Token,
		JWKSURI:               ep.JWKS,
		IntrospectionEndpoint: ep.Introspection,
		EndSessionEndpoint:    ep.EndSession,
		// Only "code" — no implicit (RFC-0006 §2 / T-005).
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported: []string{
			"authorization_code",
			"refresh_token",
			"urn:ietf:params:oauth:grant-type:device_code",
		},
		// Only S256 — "plain" is refused.
		CodeChallengeMethodsSupported:     []string{pkceMethodS256},
		SubjectTypesSupported:             []string{"public"}, // sub is opaque
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "profile", "offline_access", ScopeEmail},
		ClaimsSupported:                   []string{"iss", "sub", "aud", "exp", "iat", "org", "mid", "acr", "amr", "auth_time", "sid", "groups", "roles", "act", "pcid", "grant_ref"},
		BackchannelLogoutSupported:        true,
		BackchannelLogoutSessionSupported: true, // logout token carries sid
	}, nil
}
