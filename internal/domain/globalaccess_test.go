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

// TestGlobalAccessValidate: principal e motivo são obrigatórios; o escopo não afeta a
// validação estrutural (uma leitura self ainda exige principal + motivo).
func TestGlobalAccessValidate(t *testing.T) {
	cases := []struct {
		name    string
		access  GlobalAccess
		wantErr bool
	}{
		{"self bem-formado", GlobalAccess{Principal: "sub-1", Reason: "login", Scope: ScopeSelf}, false},
		{"cross-tenant bem-formado", GlobalAccess{Principal: "op-1", Reason: "relatório", Scope: ScopeCrossTenant}, false},
		{"sem principal", GlobalAccess{Reason: "login", Scope: ScopeSelf}, true},
		{"sem motivo", GlobalAccess{Principal: "sub-1", Scope: ScopeSelf}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.access.Validate()
			if tc.wantErr && !errors.Is(err, ErrGlobalAccessDenied) {
				t.Fatalf("esperava ErrGlobalAccessDenied, obtive %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("não esperava erro, obtive %v", err)
			}
		})
	}
}

// TestGlobalAccessScopeDefaultIsCrossTenant: o zero value do escopo é ScopeCrossTenant — o
// caminho RESTRITO (ADR-0022). Um call-site que esquece de declarar o escopo cai no
// conservador, nunca no permissivo (fail-safe).
func TestGlobalAccessScopeDefaultIsCrossTenant(t *testing.T) {
	var a GlobalAccess
	if a.Scope != ScopeCrossTenant {
		t.Fatalf("escopo default = %v, esperado ScopeCrossTenant (o restrito)", a.Scope)
	}
	if ScopeSelf == ScopeCrossTenant {
		t.Fatal("ScopeSelf não pode coincidir com ScopeCrossTenant")
	}
}
