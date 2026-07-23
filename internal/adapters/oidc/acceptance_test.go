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

package oidc

import (
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

// T-018 — "token de um componente recusado por outro (audiência)": um token
// assinado para Warpgate, verificado com sucesso pela assinatura, é RECUSADO por
// Guacamole na checagem de audiência (cenário "Reuso entre componentes").
func TestAcceptanceTokenRejectedByAudience(t *testing.T) {
	reg, err := domain.DefaultClientRegistry()
	if err != nil {
		t.Fatalf("DefaultClientRegistry: %v", err)
	}
	warpgate, _ := reg.Lookup("warpgate")
	guacamole, _ := reg.Lookup("guacamole")

	key, _ := GenerateSigningKey("kid-1")
	s, _ := NewSigner(key)
	token, err := s.Sign(sampleClaims(warpgate.Audience))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	jwks, _ := s.JWKS()

	// A ASSINATURA é válida (ambos honram o JWKS do ArchGuard)...
	claims, err := verifyAgainstJWKS(t, jwks, token)
	if err != nil {
		t.Fatalf("a assinatura deveria verificar: %v", err)
	}
	tokenAud, _ := claims["aud"].(string)

	// ...mas a AUDIÊNCIA vincula o token ao Warpgate: Guacamole recusa.
	if err := domain.ValidateAudience(tokenAud, guacamole.Audience); !errors.Is(err, domain.ErrAudienceMismatch) {
		t.Fatalf("Guacamole deveria recusar um token de Warpgate por audiência: %v", err)
	}
	// Warpgate aceita.
	if err := domain.ValidateAudience(tokenAud, warpgate.Audience); err != nil {
		t.Fatalf("Warpgate deveria aceitar o próprio token: %v", err)
	}
}
