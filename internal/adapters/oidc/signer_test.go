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
	"encoding/json"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

func sampleClaims(aud string) domain.OIDCClaims {
	return domain.OIDCClaims{
		Issuer: "https://archguard.example", Subject: "sub-opaque", Audience: aud,
		ExpiresAt: 1800000900, IssuedAt: 1800000000,
		Organization: "org", MembershipID: "mid", ACR: "L2", AMR: []string{"pwd", "webauthn"},
		AuthTime: 1800000000, SessionID: "sid", ClaimsVersion: domain.OIDCClaimsVersion,
	}
}

// verifyAgainstJWKS parses token against the published JWKS, honoring its kid.
// Returns the parsed claims or an error (mirrors a component's validation).
func verifyAgainstJWKS(t *testing.T, jwksJSON []byte, token string) (jwt.MapClaims, error) {
	t.Helper()
	var set jose.JSONWebKeySet
	if err := json.Unmarshal(jwksJSON, &set); err != nil {
		t.Fatalf("parse JWKS: %v", err)
	}
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (interface{}, error) {
		kid, _ := tok.Header["kid"].(string)
		keys := set.Key(kid)
		if len(keys) == 0 {
			// kid desconhecido: um componente renovaria o JWKS antes de rejeitar.
			return nil, jwt.ErrTokenUnverifiable
		}
		return keys[0].Key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	return claims, err
}

// Assina e valida pelo kid: o token assinado é verificável contra o JWKS
// publicado (cenário base da federação).
func TestSignAndVerify(t *testing.T) {
	key, err := GenerateSigningKey("kid-1")
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}
	s, err := NewSigner(key)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	token, err := s.Sign(sampleClaims("warpgate"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	jwks, err := s.JWKS()
	if err != nil {
		t.Fatalf("JWKS: %v", err)
	}
	claims, err := verifyAgainstJWKS(t, jwks, token)
	if err != nil {
		t.Fatalf("verificação deveria passar: %v", err)
	}
	if claims["org"] != "org" || claims["acr"] != "L2" {
		t.Fatalf("claims verificados inesperados: %v", claims)
	}
}

// Rotação com sobreposição: um token assinado ANTES da rotação continua válido
// (o JWKS publica a chave nova E a anterior); um token novo também (cenário
// "Rotação com sobreposição").
func TestRotationOverlap(t *testing.T) {
	keyA, _ := GenerateSigningKey("kid-A")
	s, _ := NewSigner(keyA)
	tokenOld, _ := s.Sign(sampleClaims("warpgate"))

	keyB, _ := GenerateSigningKey("kid-B")
	if err := s.Rotate(keyB, 2); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if s.CurrentKID() != "kid-B" {
		t.Fatalf("a chave corrente deveria ser kid-B, é %s", s.CurrentKID())
	}
	tokenNew, _ := s.Sign(sampleClaims("warpgate"))

	jwks, _ := s.JWKS()
	if _, err := verifyAgainstJWKS(t, jwks, tokenOld); err != nil {
		t.Fatalf("token anterior deveria continuar válido na sobreposição: %v", err)
	}
	if _, err := verifyAgainstJWKS(t, jwks, tokenNew); err != nil {
		t.Fatalf("token novo deveria ser válido: %v", err)
	}
}

// kid desconhecido: um token assinado por uma chave fora do JWKS não valida (o
// componente renovaria o cache do JWKS antes de rejeitar — cenário "kid
// desconhecido").
func TestUnknownKID(t *testing.T) {
	keyA, _ := GenerateSigningKey("kid-A")
	s, _ := NewSigner(keyA)
	jwks, _ := s.JWKS() // só kid-A

	// Token assinado por outra chave (kid-Z), ausente do JWKS.
	keyZ, _ := GenerateSigningKey("kid-Z")
	other, _ := NewSigner(keyZ)
	tokenZ, _ := other.Sign(sampleClaims("warpgate"))

	if _, err := verifyAgainstJWKS(t, jwks, tokenZ); err == nil {
		t.Fatalf("token com kid ausente do JWKS não deveria validar")
	}
}

// O logout token assinado é verificável, tem typ logout+jwt e carrega o evento.
func TestSignLogoutToken(t *testing.T) {
	key, _ := GenerateSigningKey("kid-1")
	s, _ := NewSigner(key)
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	claims, err := domain.NewLogoutTokenClaims("iss", "warpgate", "sid-1", "jti-1", at)
	if err != nil {
		t.Fatalf("NewLogoutTokenClaims: %v", err)
	}
	token, err := s.SignLogoutToken(claims)
	if err != nil {
		t.Fatalf("SignLogoutToken: %v", err)
	}
	jwks, _ := s.JWKS()
	verified, err := verifyAgainstJWKS(t, jwks, token)
	if err != nil {
		t.Fatalf("logout token deveria verificar: %v", err)
	}
	if verified["sid"] != "sid-1" {
		t.Fatalf("sid = %v, quero sid-1", verified["sid"])
	}
	events, ok := verified["events"].(map[string]interface{})
	if !ok || events[domain.BackchannelLogoutEvent] == nil {
		t.Fatalf("o evento de back-channel logout deveria estar no token: %v", verified["events"])
	}
}
