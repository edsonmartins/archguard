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
	"sort"
	"strings"
)

// OpenBaoPolicyPrefix namespaces every policy ArchGuard maps a role to, so a
// vault policy generated from an ArchGuard role never collides with a
// hand-written one.
const OpenBaoPolicyPrefix = "archguard-"

// OpenBaoPolicyForRole maps ONE ArchGuard role to its OpenBao policy name,
// deterministically (T-014). The mapping is a pure function of the role token, so
// the vault policy a session receives is generated from the SAME source of roles
// as the token's `roles` claim — the two cannot drift (RFC-0006 §9 risk
// mitigation). The role is lowercased and its separators normalized to a stable
// policy id.
func OpenBaoPolicyForRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	r = strings.ReplaceAll(r, " ", "-")
	r = strings.ReplaceAll(r, "_", "-")
	return OpenBaoPolicyPrefix + r
}

// OpenBaoPoliciesForRoles maps a session's roles to the OpenBao policies it
// receives, deduplicated and sorted (deterministic). Empty roles are ignored.
func OpenBaoPoliciesForRoles(roles []string) []string {
	seen := make(map[string]bool, len(roles))
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		if strings.TrimSpace(role) == "" {
			continue
		}
		p := OpenBaoPolicyForRole(role)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// OpenBaoJWTConfig is the config of OpenBao's JWT/OIDC auth method against
// ArchGuard (RFC-0006 §2 / T-014). It binds the mount to the ArchGuard issuer and
// the openbao audience, reads the identity from `sub`, and derives the vault
// policies from the `roles` claim — the group claim OpenBao maps through
// OpenBaoPoliciesForRoles. Every field comes from the claims contract, so the
// mount configuration is generated, not hand-maintained.
type OpenBaoJWTConfig struct {
	BoundIssuer    string
	BoundAudiences []string
	UserClaim      string
	GroupsClaim    string
	JWKSURL        string
}

// NewOpenBaoJWTConfig derives the mount config from the issuer, the openbao
// client audience and the JWKS URL.
func NewOpenBaoJWTConfig(issuer, openbaoAudience, jwksURL string) (OpenBaoJWTConfig, error) {
	if issuer == "" || openbaoAudience == "" || jwksURL == "" {
		return OpenBaoJWTConfig{}, fmt.Errorf("%w: issuer/audience/jwks obrigatórios", ErrInvalidClaims)
	}
	return OpenBaoJWTConfig{
		BoundIssuer:    issuer,
		BoundAudiences: []string{openbaoAudience},
		UserClaim:      "sub",   // opaque subject
		GroupsClaim:    "roles", // the roles claim drives the policy mapping
		JWKSURL:        jwksURL,
	}, nil
}
