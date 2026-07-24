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
	"testing"
	"time"

	"github.com/google/uuid"
)

// pdpDown is a PDP that never reaches a verdict — every method fails, modelling an
// unreachable OpenFGA/engine.
type pdpDown struct{}

func (pdpDown) Check(context.Context, CheckRequest) (Decision, error) {
	return Decision{}, ErrPDPUnavailable
}
func (pdpDown) ListObjects(context.Context, ListObjectsRequest) ([]string, error) {
	return nil, ErrPDPUnavailable
}
func (pdpDown) Write(context.Context, []TupleUpdate) error { return ErrPDPUnavailable }
func (pdpDown) Read(context.Context, TupleFilter) ([]RelationTuple, error) {
	return nil, ErrPDPUnavailable
}

// PDP fora do ar (T-018 / spec "PDP fora do ar"): a decisão privilegiada é NEGADA
// (fail-closed), com outcome `error` distinto de `denied`.
func TestPrivilegedDecisionDeniedWhenPDPDown(t *testing.T) {
	var pdp PolicyDecisionPoint = pdpDown{}
	org := uuid.New()
	req := CheckRequest{
		Tuple: RelationTuple{
			User:     Qualify(org, TypeMembership, "m"),
			Relation: RelCanOpenPrivilegedSession,
			Object:   Qualify(org, TypeAsset, "cofre"),
		},
		Context: CheckContext{ACR: "L2", EvaluatedAt: time.Now()},
	}
	dec, err := pdp.Check(context.Background(), req)
	granted, outcome := DecisionOutcome(dec, err)
	if granted {
		t.Fatalf("com o PDP fora do ar a privilegiada JAMAIS deveria abrir")
	}
	if outcome != Failed {
		t.Fatalf("o outcome deveria ser Failed (error), distinto de Denied, veio %v", outcome)
	}
}

// AuthN e emissão de token permanecem funcionais sem o PDP: a construção da sessão
// e das claims OIDC não tem qualquer dependência do ponto de decisão.
func TestAuthNAndTokenWorkWithoutPDP(t *testing.T) {
	identityID := uuid.New()
	orgID := uuid.New()
	mem, err := NewMembership(identityID, orgID)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	session, err := NewAuthSession(identityID, AAL2, []Membership{mem})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	at := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	if err := session.SetAuthContext(at, []FactorType{FactorPassword, FactorTOTP}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}

	// Emissão de token sem NENHUM PDP envolvido — AuthN é independente (I-1.3).
	claims, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer:    "https://archguard.example",
		Audience:  "warpgate",
		Subject:   "subj-opaco",
		Session:   &session,
		IssuedAt:  at.Add(time.Minute),
		AccessTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("a emissão de token deveria funcionar sem o PDP: %v", err)
	}
	if claims.Organization != orgID.String() {
		t.Fatalf("claims da sessão inesperados: %+v", claims)
	}
}
