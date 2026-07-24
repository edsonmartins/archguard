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
	"time"

	"github.com/google/uuid"
)

// T-015: L3 NUNCA degrada — cofre indisponível => negado. Não-L3 continua breve
// no cache; cache expirado => negado.
func TestSigningAvailability(t *testing.T) {
	cases := []struct {
		name                string
		healthy, fresh      bool
		level               AssuranceLevel
		wantAllow, wantDegr bool
	}{
		{"cofre saudável L3", true, false, L3, true, false},
		{"cofre saudável L1", true, false, L1, true, false},
		{"cofre down L3 => negado", false, true, L3, false, false},
		{"cofre down L2 cache fresco => degradado", false, true, L2, true, true},
		{"cofre down L1 cache expirado => negado", false, false, L1, false, false},
	}
	for _, c := range cases {
		allow, degr := SigningAvailability(c.healthy, c.fresh, c.level)
		if allow != c.wantAllow || degr != c.wantDegr {
			t.Fatalf("%s: allow=%v degr=%v; queria allow=%v degr=%v", c.name, allow, degr, c.wantAllow, c.wantDegr)
		}
	}
}

// T-013: a sobreposição da rotação de JWKS deve ser >= TTL máximo de token.
func TestValidateRotationOverlap(t *testing.T) {
	if err := ValidateRotationOverlap(2*time.Hour, time.Hour); err != nil {
		t.Fatalf("sobreposição maior que o TTL deveria passar: %v", err)
	}
	if err := ValidateRotationOverlap(30*time.Minute, time.Hour); !errors.Is(err, ErrOverlapTooShort) {
		t.Fatalf("sobreposição menor que o TTL deveria falhar, veio %v", err)
	}
	// Rotação é L3 e auditada.
	in := BuildKeyRotationAuditInput(uuid.New(), "admin-opaco", "jwks:v2", "jwks")
	if in.Action != ActionKeyRotate || in.Context.AuthContextClass != "L3" {
		t.Fatalf("auditoria de rotação deveria ser key.rotate L3: %+v", in)
	}
	if ActionKeyRotate.AssuranceLevel() != L3 {
		t.Fatalf("key.rotate deveria ser L3")
	}
}

// T-014: um selo é verificável contra a versão de chave válida no seu tempo; a
// rotação preserva a verificabilidade dos selos antigos.
func TestSealKeyValidityRotation(t *testing.T) {
	reg := NewSealKeyRegistry()
	t0 := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	reg.Register(SealKeyValidity{KeyID: "seal:v1", NotBefore: t0})

	// Um selo assinado por v1 após t0 é válido.
	if !reg.ValidForSeal("seal:v1", t0.Add(time.Hour)) {
		t.Fatalf("selo por v1 após o início deveria ser válido")
	}
	// Rotaciona em t1: v1 fecha, v2 abre.
	t1 := t0.Add(24 * time.Hour)
	reg.Rotate("seal:v1", "seal:v2", t1)

	// Selo ANTIGO (por v1, antes de t1) ainda verifica — a rotação não o invalida.
	if !reg.ValidForSeal("seal:v1", t0.Add(2*time.Hour)) {
		t.Fatalf("selo antigo por v1 deveria continuar verificável após rotação")
	}
	// Um selo por v1 DEPOIS de t1 é inválido (v1 já saiu de vigência = adulteração).
	if reg.ValidForSeal("seal:v1", t1.Add(time.Hour)) {
		t.Fatalf("selo por v1 após a rotação deveria ser inválido")
	}
	// Selo por v2 após t1 é válido.
	if !reg.ValidForSeal("seal:v2", t1.Add(time.Hour)) {
		t.Fatalf("selo por v2 após a rotação deveria ser válido")
	}
	// key_id desconhecido => recusado (fail-closed).
	if reg.ValidForSeal("seal:v9", t1) {
		t.Fatalf("key_id desconhecido deveria ser recusado")
	}
}
