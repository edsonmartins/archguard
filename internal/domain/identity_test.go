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

func TestNewIdentityDefaults(t *testing.T) {
	id, err := NewIdentity(IdentityHuman)
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if id.Type != IdentityHuman {
		t.Errorf("type = %q, quer human", id.Type)
	}
	if id.Status != IdentityActive {
		t.Errorf("status = %q, quer active (nasce ativa)", id.Status)
	}
	if id.ID.Version() != 7 {
		t.Errorf("id deveria ser UUIDv7, veio versão %d", id.ID.Version())
	}
	if id.Subject == "" {
		t.Error("subject vazio — deveria ser material opaco")
	}
	// Subject é opaco e independente do id: não pode ser a string do UUID (que
	// vazaria a ordenação temporal do v7).
	if id.Subject == id.ID.String() {
		t.Error("subject não pode ser o próprio id (vazaria tempo de criação)")
	}
	// Campos pessoais nascem vazios — populados por camada com KeyCustodian.
	if id.PrimaryEmailEnc != nil || id.EmailHash != nil || id.DisplayNameEnc != nil {
		t.Error("campos pessoais deveriam nascer vazios no domínio")
	}
}

func TestNewIdentityServiceType(t *testing.T) {
	id, err := NewIdentity(IdentityService)
	if err != nil {
		t.Fatalf("NewIdentity(service): %v", err)
	}
	if id.Type != IdentityService {
		t.Errorf("type = %q, quer service", id.Type)
	}
}

func TestNewIdentityRejectsInvalidType(t *testing.T) {
	for _, bad := range []IdentityType{"", "robot", "Human", "HUMAN"} {
		if _, err := NewIdentity(bad); !errors.Is(err, ErrInvalidIdentityType) {
			t.Errorf("NewIdentity(%q) erro = %v, quer ErrInvalidIdentityType", bad, err)
		}
	}
}

func TestNewIdentitySubjectsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	ids := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := NewIdentity(IdentityHuman)
		if err != nil {
			t.Fatalf("NewIdentity: %v", err)
		}
		if seen[id.Subject] {
			t.Fatalf("subject repetido: %q", id.Subject)
		}
		if ids[id.ID.String()] {
			t.Fatalf("id repetido: %q", id.ID)
		}
		seen[id.Subject] = true
		ids[id.ID.String()] = true
	}
}

func TestIdentitySuspendReactivate(t *testing.T) {
	id, _ := NewIdentity(IdentityHuman)
	if err := id.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if id.Status != IdentitySuspended {
		t.Errorf("status = %q, quer suspended", id.Status)
	}
	if err := id.Reactivate(); err != nil {
		t.Fatalf("Reactivate: %v", err)
	}
	if id.Status != IdentityActive {
		t.Errorf("status = %q, quer active", id.Status)
	}
}

func TestIdentityDeprovisionIsTerminal(t *testing.T) {
	id, _ := NewIdentity(IdentityHuman)
	id.Deprovision()
	if id.Status != IdentityDeprovisioned {
		t.Fatalf("status = %q, quer deprovisioned", id.Status)
	}
	// R5: estado terminal — nenhuma transição sai dele.
	if err := id.Suspend(); !errors.Is(err, ErrIdentityDeprovisioned) {
		t.Errorf("Suspend após deprovision erro = %v, quer ErrIdentityDeprovisioned", err)
	}
	if err := id.Reactivate(); !errors.Is(err, ErrIdentityDeprovisioned) {
		t.Errorf("Reactivate após deprovision erro = %v, quer ErrIdentityDeprovisioned", err)
	}
	// Idempotente.
	id.Deprovision()
	if id.Status != IdentityDeprovisioned {
		t.Errorf("Deprovision não idempotente: %q", id.Status)
	}
}

func TestIdentityTypeAndStatusValid(t *testing.T) {
	for _, ty := range []IdentityType{IdentityHuman, IdentityService} {
		if !ty.Valid() {
			t.Errorf("%q deveria ser válido", ty)
		}
	}
	for _, ty := range []IdentityType{"", "robot"} {
		if ty.Valid() {
			t.Errorf("%q não deveria ser válido", ty)
		}
	}
	for _, s := range []IdentityStatus{IdentityActive, IdentitySuspended, IdentityDeprovisioned} {
		if !s.Valid() {
			t.Errorf("%q deveria ser válido", s)
		}
	}
	for _, s := range []IdentityStatus{"", "deleted"} {
		if s.Valid() {
			t.Errorf("%q não deveria ser válido", s)
		}
	}
}
