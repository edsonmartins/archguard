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

// O mapa role -> política é determinístico e vem da MESMA fonte de papéis (a
// política do cofre não pode divergir do papel do token).
func TestOpenBaoPolicyMapping(t *testing.T) {
	if OpenBaoPolicyForRole("DB Admin") != "archguard-db-admin" {
		t.Fatalf("mapeamento inesperado: %q", OpenBaoPolicyForRole("DB Admin"))
	}
	// Determinístico: mesmo papel -> mesma política.
	if OpenBaoPolicyForRole("operator") != OpenBaoPolicyForRole("operator") {
		t.Fatalf("o mapeamento deveria ser determinístico")
	}

	pols := OpenBaoPoliciesForRoles([]string{"operator", "db_admin", "", "operator"})
	// Dedup + ordenado; ignora vazio.
	want := []string{"archguard-db-admin", "archguard-operator"}
	if len(pols) != len(want) {
		t.Fatalf("políticas = %v, quero %v", pols, want)
	}
	for i := range want {
		if pols[i] != want[i] {
			t.Fatalf("políticas[%d] = %q, quero %q (%v)", i, pols[i], want[i], pols)
		}
	}
}

// A config do auth method JWT do OpenBao é derivada do contrato: sub como
// identidade, roles como grupos, audiência/issuer vinculados.
func TestOpenBaoJWTConfig(t *testing.T) {
	cfg, err := NewOpenBaoJWTConfig("https://archguard.example", "openbao", "https://archguard.example/jwks")
	if err != nil {
		t.Fatalf("NewOpenBaoJWTConfig: %v", err)
	}
	if cfg.UserClaim != "sub" || cfg.GroupsClaim != "roles" {
		t.Fatalf("config inesperada: %+v", cfg)
	}
	if len(cfg.BoundAudiences) != 1 || cfg.BoundAudiences[0] != "openbao" {
		t.Fatalf("a audiência vinculada deveria ser openbao: %+v", cfg.BoundAudiences)
	}
	if _, err := NewOpenBaoJWTConfig("", "openbao", "jwks"); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("config sem issuer deveria falhar: %v", err)
	}
}
