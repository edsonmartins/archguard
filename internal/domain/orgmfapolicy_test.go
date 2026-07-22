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

func TestNewOrgMFAPolicy(t *testing.T) {
	org := uuid.New()
	p, err := NewOrgMFAPolicy(org, AAL2)
	if err != nil {
		t.Fatalf("NewOrgMFAPolicy: %v", err)
	}
	if p.OrganizationID != org || p.MinimumAAL != AAL2 {
		t.Fatalf("política inesperada: %+v", p)
	}
	if p.RequiresPhishingResistant() {
		t.Fatalf("AAL2 não deveria exigir phishing-resistant")
	}

	// AAL3 = WebAuthn obrigatório.
	web, _ := NewOrgMFAPolicy(org, AAL3)
	if !web.RequiresPhishingResistant() {
		t.Fatalf("AAL3 deveria exigir phishing-resistant")
	}

	// Inválidas.
	if _, err := NewOrgMFAPolicy(uuid.Nil, AAL2); !errors.Is(err, ErrInvalidOrgMFAPolicy) {
		t.Fatalf("org nula: err = %v", err)
	}
	if _, err := NewOrgMFAPolicy(org, AAL("nope")); !errors.Is(err, ErrInvalidOrgMFAPolicy) {
		t.Fatalf("nível inválido: err = %v", err)
	}
}

func TestDefaultOrgMFAPolicy(t *testing.T) {
	org := uuid.New()
	p := DefaultOrgMFAPolicy(org)
	if p.MinimumAAL != DefaultOrgMinimumAAL {
		t.Fatalf("baseline deveria ser o default, veio %s", p.MinimumAAL)
	}
	if DefaultOrgMinimumAAL != AAL1 {
		t.Fatalf("o baseline da plataforma deveria ser AAL1, é %s", DefaultOrgMinimumAAL)
	}
}

func TestOrgMFAPolicySatisfiedBy(t *testing.T) {
	org := uuid.New()
	web, _ := NewOrgMFAPolicy(org, AAL3)

	if web.SatisfiedBy(AAL2) {
		t.Fatalf("AAL2 não deveria satisfazer floor AAL3 (cenário TOTP→tenant WebAuthn)")
	}
	if !web.SatisfiedBy(AAL3) {
		t.Fatalf("AAL3 deveria satisfazer floor AAL3")
	}
	// Fail-closed: nível indefinido não satisfaz nada.
	if web.SatisfiedBy(AAL("")) {
		t.Fatalf("nível indefinido não deveria satisfazer nenhum floor")
	}
}
