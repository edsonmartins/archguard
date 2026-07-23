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

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/globalaccess"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
)

// withDevProfile allows the authorized cross-tenant read the bridge performs (the
// provisional ProfileAuthorizer permits it only under the dev profile; the real
// authorizer is the OpenFGA PDP of pacote 007).
func withDevProfile(t *testing.T) {
	t.Helper()
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })
	deploy.SetActive(deploy.Dev)
}

// A ponte estabelece um auth_session ativo a partir do login (membership único),
// e o carrega depois — a sessão criada serve de base para o token OIDC.
func TestSessionBridgeEstablishAndLoad(t *testing.T) {
	pool := setupTenantPool(t)
	withDevProfile(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "bridge")

	global := NewGlobalRepository(pool, globalaccess.NewProfileAuthorizer(), globalaccess.NewMemoryAuditor())
	bridge := NewSessionBridge(pool, global)

	at := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	// fx.other tem exatamente um membership ativo (otherMemA) -> sessão nasce ativa.
	session, err := bridge.EstablishSession(ctx, fx.other, domain.AAL2,
		[]domain.FactorType{domain.FactorPassword, domain.FactorTOTP}, at)
	if err != nil {
		t.Fatalf("EstablishSession: %v", err)
	}
	if session.Status != domain.SessionActive {
		t.Fatalf("com um membership ativo a sessão deveria nascer ativa, veio %s", session.Status)
	}
	mem, org, err := session.ActiveTenant()
	if err != nil || mem != fx.otherMemA.ID || org != fx.orgA {
		t.Fatalf("tenant ativo inesperado: (%s,%s) err=%v", mem, org, err)
	}

	// Recarrega pela ponte (hot path do resolver).
	loaded, err := bridge.LoadSession(ctx, fx.other.ID, session.ID)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.ACR() != "L2" || loaded.Status != domain.SessionActive {
		t.Fatalf("sessão recarregada inesperada: acr=%q status=%s", loaded.ACR(), loaded.Status)
	}

	// A partir da sessão criada, um token OIDC pode ser montado (o objetivo do
	// login real).
	claims, err := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
		Issuer: "https://archguard.example", Audience: "warpgate", Subject: fx.other.Subject,
		Session: &loaded, IssuedAt: at.Add(time.Minute), AccessTTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("o login real deveria permitir montar o token: %v", err)
	}
	if claims.Organization != fx.orgA.String() || claims.ACR != "L2" {
		t.Fatalf("claims do token da sessão bridged inesperados: %+v", claims)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM auth_session WHERE id = $1", session.ID.String())
	})
}

// Identidade sem membership ativo: negada (não há sessão sem contexto de tenant).
func TestSessionBridgeNoActiveMembership(t *testing.T) {
	pool := setupTenantPool(t)
	withDevProfile(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "bridgeno")

	// Cria uma identidade SEM membership.
	lone := seedRecoveryIdentity(t, pool)
	loneIdentity, err := NewIdentityStore(pool).Get(ctx, lone)
	if err != nil {
		t.Fatalf("Get identidade: %v", err)
	}

	global := NewGlobalRepository(pool, globalaccess.NewProfileAuthorizer(), globalaccess.NewMemoryAuditor())
	bridge := NewSessionBridge(pool, global)

	if _, err := bridge.EstablishSession(ctx, loneIdentity, domain.AAL1,
		[]domain.FactorType{domain.FactorPassword}, time.Now()); !errors.Is(err, domain.ErrNoActiveMembership) {
		t.Fatalf("identidade sem membership deveria ser negada: %v", err)
	}
	_ = fx
}
