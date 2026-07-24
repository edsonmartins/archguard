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

import "testing"

// spec "Estado padrão": numa instalação nova, o canal legado (RADIUS embutido)
// está DESABILITADO. Só um afirmativo explícito o habilita.
func TestLegacyChannelDisabledByDefault(t *testing.T) {
	// Ausente/vazio/negativo/malformado => DESABILITADO.
	for _, flag := range []string{"", "  ", "false", "0", "no", "off", "talvez", "disabled"} {
		if NewLegacyChannelConfig(flag).Enabled(LegacyRADIUS) {
			t.Fatalf("flag %q NÃO deveria habilitar o RADIUS embutido", flag)
		}
	}
	// Afirmativos explícitos => habilitado.
	for _, flag := range []string{"true", "TRUE", "1", "yes", "on", "Enabled", " true "} {
		if !NewLegacyChannelConfig(flag).Enabled(LegacyRADIUS) {
			t.Fatalf("flag %q deveria habilitar o RADIUS embutido", flag)
		}
	}
}

// O zero-value tem tudo desabilitado (default seguro).
func TestLegacyChannelZeroValueDisabled(t *testing.T) {
	var cfg LegacyChannelConfig
	if cfg.Enabled(LegacyRADIUS) {
		t.Fatalf("zero-value deveria ter o RADIUS desabilitado")
	}
	// Canal desconhecido nunca habilita.
	if cfg.Enabled(LegacyChannel("telnet")) {
		t.Fatalf("canal desconhecido nunca deveria estar habilitado")
	}
}
