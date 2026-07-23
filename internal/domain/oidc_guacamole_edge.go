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
	"fmt"
	"time"
)

// GuacamoleEdgeConfig is the configuration of the EDGE adaptation in front of
// Apache Guacamole (T-015 / RFC-0006 §9). Guacamole's OIDC extension has limited
// claim and logout support; the edge compensates WITHOUT degrading the central
// contract (design 006 §"Adaptação sem contaminação"): it introspects with a
// short TTL to propagate revocation (no back-channel logout), enforces acr per
// operation, and translates the contract claims to the form the extension reads.
// Every field is derived from the client registry and the claims contract — the
// edge invents nothing.
type GuacamoleEdgeConfig struct {
	Audience string
	// IntrospectionTTL is the short cache the edge honors — its compensation for
	// the missing back-channel logout.
	IntrospectionTTL time.Duration
	// EnforceACR is always true: the edge refuses insufficient-assurance requests
	// and redirects to step-up, since the Guacamole extension does not enforce acr
	// itself.
	EnforceACR bool
	// TranslatedClaims are the contract claims the edge maps to the extension's
	// expected fields — read from the contract, never emitted outside it.
	TranslatedClaims []string
}

// NewGuacamoleEdgeConfig derives the edge config from the Guacamole client entry
// of the registry. It refuses a client that is not Guacamole (the edge is
// component-specific) or one that unexpectedly declares back-channel logout (the
// edge exists precisely because Guacamole does not).
func NewGuacamoleEdgeConfig(client OIDCClient) (GuacamoleEdgeConfig, error) {
	if client.ClientID != "guacamole" {
		return GuacamoleEdgeConfig{}, fmt.Errorf("%w: adaptação de borda é específica do Guacamole", ErrUnknownClient)
	}
	if client.SupportsBackchannelLogout() {
		return GuacamoleEdgeConfig{}, fmt.Errorf("%w: a borda pressupõe ausência de back-channel logout", ErrUnknownClient)
	}
	return GuacamoleEdgeConfig{
		Audience:         client.Audience,
		IntrospectionTTL: RecommendedIntrospectionTTL,
		EnforceACR:       true,
		TranslatedClaims: []string{"org", "mid", "acr", "roles", "groups", "pcid"},
	}, nil
}
