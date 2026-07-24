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
	"strings"
	"testing"
)

// spec "Operação privilegiada por canal legado": uma sessão por RADIUS NÃO
// autoriza operações L3 e é sinalizada como canal legado.
func TestLegacyChannelSessionNeverL3(t *testing.T) {
	s := LegacyChannelSession{Channel: LegacyRADIUS}

	if L3.Satisfies(s.ProvenAAL(), s.PhishingResistant()) {
		t.Fatalf("sessão de canal legado NÃO deveria satisfazer L3")
	}
	if s.AuthorizesL3() {
		t.Fatalf("AuthorizesL3 deveria ser false para canal legado")
	}
	// Também não alcança L2 (só identificação L1).
	if L2.Satisfies(s.ProvenAAL(), s.PhishingResistant()) {
		t.Fatalf("canal legado não deveria satisfazer L2")
	}
	if !L1.Satisfies(s.ProvenAAL(), s.PhishingResistant()) {
		t.Fatalf("canal legado deveria estabelecer identificação L1")
	}
}

// O acesso por canal legado é sinalizado como tal na auditoria (sem PII).
func TestLegacyChannelAuditFlag(t *testing.T) {
	flag := LegacyChannelSession{Channel: LegacyRADIUS}.AuditFlag()
	if !strings.Contains(flag, "legacy") || !strings.Contains(flag, "radius") {
		t.Fatalf("flag de auditoria deveria identificar o canal legado, veio %q", flag)
	}
}
