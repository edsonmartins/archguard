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
)

// O registro padrão traz os cinco componentes do ArchGate, cada um com sua
// audiência.
func TestDefaultClientRegistry(t *testing.T) {
	reg, err := DefaultClientRegistry()
	if err != nil {
		t.Fatalf("DefaultClientRegistry: %v", err)
	}
	for _, id := range []string{"warpgate", "guacamole", "netbird", "openbao", "oracle-jdbc-proxy"} {
		c, err := reg.Lookup(id)
		if err != nil {
			t.Fatalf("cliente %q deveria estar registrado: %v", id, err)
		}
		if c.Audience != id {
			t.Fatalf("audiência de %q deveria ser %q, veio %q", id, id, c.Audience)
		}
	}
	if len(reg.IDs()) != 5 {
		t.Fatalf("deveria haver 5 clientes, há %d", len(reg.IDs()))
	}
}

// Perfis por componente (RFC-0006 §2): NetBird tem device flow; Warpgate tem
// back-channel logout; Guacamole não; Oracle proxy só valida JWT.
func TestClientProfiles(t *testing.T) {
	reg, _ := DefaultClientRegistry()

	netbird, _ := reg.Lookup("netbird")
	if !netbird.AllowsFlow(FlowDeviceCode) {
		t.Fatalf("NetBird deveria permitir device flow")
	}

	warpgate, _ := reg.Lookup("warpgate")
	if !warpgate.SupportsBackchannelLogout() {
		t.Fatalf("Warpgate deveria ter back-channel logout")
	}

	guac, _ := reg.Lookup("guacamole")
	if guac.SupportsBackchannelLogout() {
		t.Fatalf("Guacamole não tem back-channel logout confiável (introspecção)")
	}
	if guac.AllowsFlow(FlowDeviceCode) {
		t.Fatalf("Guacamole não deveria permitir device flow")
	}
}

// A validação de fluxo por cliente: device flow num cliente que não o permite é
// recusado; cliente desconhecido também.
func TestAuthorizeClientFlow(t *testing.T) {
	reg, _ := DefaultClientRegistry()

	if _, err := reg.AuthorizeClientFlow("warpgate", FlowAuthorizationCode); err != nil {
		t.Fatalf("Warpgate deveria permitir Authorization Code: %v", err)
	}
	if _, err := reg.AuthorizeClientFlow("warpgate", FlowDeviceCode); !errors.Is(err, ErrFlowNotAllowedForClient) {
		t.Fatalf("Warpgate não deveria permitir device flow: %v", err)
	}
	if _, err := reg.AuthorizeClientFlow("desconhecido", FlowAuthorizationCode); !errors.Is(err, ErrUnknownClient) {
		t.Fatalf("cliente desconhecido deveria ser recusado: %v", err)
	}
}
