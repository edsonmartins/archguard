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

package globalaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
)

// TestScopedAuthorizerSelfAllowedInEveryProfile: acesso self-confinado (login/console
// lendo os próprios memberships) é permitido em QUALQUER perfil (ADR-0022) — o gate de
// login não pode depender de perfil/serviço externo (I-1.3).
func TestScopedAuthorizerSelfAllowedInEveryProfile(t *testing.T) {
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })

	a := NewScopedAuthorizer()
	self := domain.GlobalAccess{Principal: "sub-1", Reason: "login: próprios memberships", Scope: domain.ScopeSelf}

	for _, p := range []deploy.Profile{deploy.Dev, deploy.Pilot, deploy.Production} {
		deploy.SetActive(p)
		if err := a.Authorize(context.Background(), self); err != nil {
			t.Errorf("perfil %v: self deveria ser permitido, veio: %v", p, err)
		}
	}
}

// TestScopedAuthorizerCrossTenantFailClosedInConformant: leitura cross-tenant ampla é
// permitida só em dev; em perfil conforme é fail-closed (INV-6) até haver política real.
func TestScopedAuthorizerCrossTenantFailClosedInConformant(t *testing.T) {
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })

	a := NewScopedAuthorizer()
	broad := domain.GlobalAccess{Principal: "op", Reason: "relatório global", Scope: domain.ScopeCrossTenant}

	deploy.SetActive(deploy.Dev)
	if err := a.Authorize(context.Background(), broad); err != nil {
		t.Errorf("dev deveria permitir cross-tenant, veio: %v", err)
	}
	for _, p := range []deploy.Profile{deploy.Pilot, deploy.Production} {
		deploy.SetActive(p)
		if err := a.Authorize(context.Background(), broad); !errors.Is(err, domain.ErrGlobalAccessDenied) {
			t.Errorf("perfil %v: cross-tenant amplo deveria negar, veio: %v", p, err)
		}
	}
}

// TestScopedAuthorizerRejectsIllFormed: principal/motivo obrigatórios, mesmo self, mesmo dev.
func TestScopedAuthorizerRejectsIllFormed(t *testing.T) {
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })
	deploy.SetActive(deploy.Dev)

	a := NewScopedAuthorizer()
	if err := a.Authorize(context.Background(), domain.GlobalAccess{Scope: domain.ScopeSelf, Reason: "sem principal"}); !errors.Is(err, domain.ErrGlobalAccessDenied) {
		t.Error("self sem principal deveria ser negado")
	}
}
