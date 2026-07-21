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

	"github.com/google/uuid"
)

func TestNewTenantScope(t *testing.T) {
	org := credID(t)
	s, err := NewTenantScope(org)
	if err != nil {
		t.Fatalf("NewTenantScope: %v", err)
	}
	if s.OrganizationID() != org {
		t.Errorf("OrganizationID = %v, quer %v", s.OrganizationID(), org)
	}
}

func TestNewTenantScopeRejectsNil(t *testing.T) {
	if _, err := NewTenantScope(uuid.Nil); !errors.Is(err, ErrNoTenant) {
		t.Errorf("tenant nulo: erro = %v, quer ErrNoTenant", err)
	}
}

func TestRLSSettingNameStable(t *testing.T) {
	// A RLS migration (T-010) fixa esta string em SQL; se ela mudar aqui sem lá,
	// as duas barreiras deixam de casar. Trava o contrato.
	if RLSOrgSettingName != "app.current_organization" {
		t.Errorf("nome do parâmetro de sessão mudou: %q", RLSOrgSettingName)
	}
}
