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
	"testing"
	"time"
)

func introspectClaims() OIDCClaims {
	return OIDCClaims{
		Issuer: "iss", Subject: "sub", Audience: "warpgate",
		IssuedAt: 1800000000, ExpiresAt: 1800000900,
		Organization: "org", MembershipID: "mid", ACR: "L2", AMR: []string{"pwd"},
		AuthTime: 1800000000, SessionID: "sid", ClaimsVersion: OIDCClaimsVersion,
	}
}

// Sessão viva e token não expirado -> active com claims. Sessão revogada ->
// active:false ANTES do token expirar (introspecção propaga revogação a
// componente sem logout).
func TestBuildIntrospection(t *testing.T) {
	claims := introspectClaims()
	now := time.Unix(1800000100, 0) // dentro da janela do token

	// Sessão viva.
	live := BuildIntrospection(claims, true, now)
	if !live.Active || live.Subject != "sub" || live.ACR != "L2" || live.SessionID != "sid" {
		t.Fatalf("token vivo deveria ser active com claims: %+v", live)
	}

	// Sessão revogada: inactive, sem vazar claims.
	revoked := BuildIntrospection(claims, false, now)
	if revoked.Active || revoked.Subject != "" || revoked.Organization != "" {
		t.Fatalf("token de sessão revogada deveria ser active:false sem claims: %+v", revoked)
	}

	// Token expirado (mesmo com sessão viva): inactive.
	expired := BuildIntrospection(claims, true, time.Unix(1800000900, 0))
	if expired.Active {
		t.Fatalf("token expirado deveria ser active:false")
	}
}
