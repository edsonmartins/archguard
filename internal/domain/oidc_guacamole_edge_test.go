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

// A borda do Guacamole compensa com introspecção de TTL curto e enforcement de
// acr, derivada do registro do cliente — sem degradar o contrato central.
func TestGuacamoleEdgeConfig(t *testing.T) {
	reg, _ := DefaultClientRegistry()
	guac, _ := reg.Lookup("guacamole")

	cfg, err := NewGuacamoleEdgeConfig(guac)
	if err != nil {
		t.Fatalf("NewGuacamoleEdgeConfig: %v", err)
	}
	if cfg.IntrospectionTTL != RecommendedIntrospectionTTL || !cfg.EnforceACR {
		t.Fatalf("a borda deveria usar introspecção curta e enforce de acr: %+v", cfg)
	}
	if cfg.Audience != "guacamole" || len(cfg.TranslatedClaims) == 0 {
		t.Fatalf("config de borda inesperada: %+v", cfg)
	}

	// A borda é específica do Guacamole: outro cliente é recusado.
	warpgate, _ := reg.Lookup("warpgate")
	if _, err := NewGuacamoleEdgeConfig(warpgate); !errors.Is(err, ErrUnknownClient) {
		t.Fatalf("a borda não deveria aceitar outro cliente: %v", err)
	}
}
