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
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Um token para o componente A é recusado pelo componente B (cenário "Reuso
// entre componentes").
func TestValidateAudience(t *testing.T) {
	if err := ValidateAudience("warpgate", "warpgate"); err != nil {
		t.Fatalf("aud igual deveria passar: %v", err)
	}
	if err := ValidateAudience("warpgate", "guacamole"); !errors.Is(err, ErrAudienceMismatch) {
		t.Fatalf("token de A em B deveria ser recusado: %v", err)
	}
	// Fail-closed: audiência vazia nunca casa.
	if err := ValidateAudience("", ""); !errors.Is(err, ErrAudienceMismatch) {
		t.Fatalf("aud vazia não deveria casar: %v", err)
	}
}

func TestMinimalScope(t *testing.T) {
	got := MinimalScope([]string{"openid", "email", "admin"}, []string{"openid", "profile", "email"})
	// Interseção de requested e allowed, na ordem de allowed: [openid email].
	if len(got) != 2 || got[0] != "openid" || got[1] != "email" {
		t.Fatalf("escopo mínimo = %v, quero [openid email]", got)
	}
}

// Dado pessoal restrito: e-mail só entra no token sob o escopo email (cenário
// "Dado pessoal restrito").
func TestEmailOnlyUnderScope(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	m, _ := NewMembership(id, org)
	s, _ := NewAuthSession(id, AAL1, []Membership{m})
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorPassword}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}

	base := OIDCClaimsInput{
		Issuer: "iss", Audience: "guacamole", Subject: "sub", Session: &s,
		IssuedAt: at, AccessTTL: 10 * time.Minute, Email: "user@example.com",
	}

	// Sem escopo email: o claim email NÃO é emitido.
	noScope, err := BuildOIDCClaims(base)
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	if noScope.Email != "" {
		t.Fatalf("e-mail não deveria vazar sem escopo, veio %q", noScope.Email)
	}

	// Com escopo email: emitido.
	base.GrantedScopes = []string{"openid", ScopeEmail}
	withScope, err := BuildOIDCClaims(base)
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	if withScope.Email != "user@example.com" {
		t.Fatalf("e-mail deveria ser emitido sob escopo, veio %q", withScope.Email)
	}
}
