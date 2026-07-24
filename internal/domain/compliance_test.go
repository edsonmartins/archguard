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

// spec "Custódia local em produção": custódia local reporta NÃO conforme, mesmo
// com o perfil marcado como conforme.
func TestComplianceCustodyLocalNeverConformant(t *testing.T) {
	// Perfil conforme + custódia LOCAL => não conforme (a custódia manda).
	local := ComplianceReport{Custody: CustodyLocal, ProfileConformant: true}
	if local.Conformant() || local.Status() != "non_conformant" {
		t.Fatalf("custódia local deveria ser não conforme mesmo com perfil conforme")
	}
	if !containsReason(local.Reasons(), "custódia") {
		t.Fatalf("o motivo deveria citar a custódia local: %v", local.Reasons())
	}

	// Cofre + perfil conforme => conforme.
	vault := ComplianceReport{Custody: CustodyVault, ProfileConformant: true}
	if !vault.Conformant() || vault.Status() != "conformant" {
		t.Fatalf("cofre + perfil conforme deveria ser conforme")
	}
	if len(vault.Reasons()) != 0 {
		t.Fatalf("instalação conforme não deveria ter motivos: %v", vault.Reasons())
	}

	// Cofre + perfil NÃO conforme => não conforme (ambos os eixos contam).
	badProfile := ComplianceReport{Custody: CustodyVault, ProfileConformant: false}
	if badProfile.Conformant() {
		t.Fatalf("perfil não conforme deveria ser não conforme")
	}
}

func TestCustodyConformant(t *testing.T) {
	if !CustodyConformant(CustodyVault) {
		t.Fatalf("cofre deveria ser conforme")
	}
	if CustodyConformant(CustodyLocal) {
		t.Fatalf("custódia local NUNCA é conforme")
	}
}

func containsReason(reasons []string, sub string) bool {
	for _, r := range reasons {
		if strings.Contains(r, sub) {
			return true
		}
	}
	return false
}
