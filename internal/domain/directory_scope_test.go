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

func TestValidateScopeFilter(t *testing.T) {
	// Válidos: filtros deliberados e delimitados.
	for _, ok := range []string{
		"(objectClass=user)",
		"(&(objectClass=user)(memberOf=CN=ArchGuard,OU=Grupos,DC=cli,DC=com))",
		"(|(department=TI)(department=Seg))",
	} {
		if err := ValidateScopeFilter(ok); err != nil {
			t.Fatalf("filtro válido rejeitado %q: %v", ok, err)
		}
	}

	// Vazio.
	if err := ValidateScopeFilter("   "); !errors.Is(err, ErrScopeFilterRequired) {
		t.Fatalf("vazio deveria ser ErrScopeFilterRequired, veio %v", err)
	}
	// Match-all (toda a árvore disfarçada).
	for _, broad := range []string{"*", "(*)", "(objectClass=*)", "( objectclass = * )"} {
		if err := ValidateScopeFilter(broad); !errors.Is(err, ErrScopeFilterTooBroad) {
			t.Fatalf("match-all %q deveria ser ErrScopeFilterTooBroad, veio %v", broad, err)
		}
	}
	// Mal-formado (parênteses desbalanceados).
	for _, bad := range []string{"(objectClass=user", "objectClass=user)", "(&(a=b)(c=d)"} {
		if err := ValidateScopeFilter(bad); !errors.Is(err, ErrScopeFilterMalformed) {
			t.Fatalf("mal-formado %q deveria ser ErrScopeFilterMalformed, veio %v", bad, err)
		}
	}
}

// A validação de escopo está amarrada à construção do conector.
func TestNewConnectorRejectsBroadScope(t *testing.T) {
	if _, err := NewDirectoryConnector(uuid.New(), DirectoryAD, "x", "(objectClass=*)", "vault://k", nil, nil); !errors.Is(err, ErrScopeFilterTooBroad) {
		t.Fatalf("conector com escopo match-all deveria ser rejeitado, veio %v", err)
	}
}
