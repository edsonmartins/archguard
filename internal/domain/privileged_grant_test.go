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

func validTarget() GrantTarget {
	return GrantTarget{Type: "asset", ID: "db-prod-01", Scope: "admin"}
}

func TestNewPrivilegedGrant(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(30 * time.Minute)

	g, err := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 2, nb, exp)
	if err != nil {
		t.Fatalf("NewPrivilegedGrant: %v", err)
	}
	if g.Status != GrantRequested || g.Origin != GrantBreakglass || g.RequiredApprovals != 2 {
		t.Fatalf("grant inesperado: %+v", g)
	}
}

func TestNewPrivilegedGrantValidation(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(time.Hour)

	cases := []struct {
		name             string
		org, sub         uuid.UUID
		target           GrantTarget
		origin           GrantOrigin
		required         int
		nb, exp          time.Time
		wantWindowErr    bool
		wantGenericError bool
	}{
		{"org nula", uuid.Nil, sub, validTarget(), GrantNormal, 1, nb, exp, false, true},
		{"subject nulo", org, uuid.Nil, validTarget(), GrantNormal, 1, nb, exp, false, true},
		{"alvo incompleto", org, sub, GrantTarget{Type: "asset"}, GrantNormal, 1, nb, exp, false, true},
		{"origem inválida", org, sub, validTarget(), GrantOrigin("x"), 1, nb, exp, false, true},
		{"aprovações negativas", org, sub, validTarget(), GrantNormal, -1, nb, exp, false, true},
		{"janela invertida", org, sub, validTarget(), GrantNormal, 1, exp, nb, true, false},
		{"janela zero", org, sub, validTarget(), GrantNormal, 1, nb, nb, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewPrivilegedGrant(c.org, c.sub, c.target, c.origin, c.required, c.nb, c.exp)
			if c.wantWindowErr && !errors.Is(err, ErrInvalidGrantWindow) {
				t.Fatalf("err = %v, quero ErrInvalidGrantWindow", err)
			}
			if c.wantGenericError && !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("err = %v, quero ErrInvalidGrant", err)
			}
		})
	}
}

// Autoridade avaliada no momento da decisão: um grant ativo autoriza só dentro
// da janela; fora dela (ou em qualquer outro status) não autoriza — base do
// cenário "Token emitido antes da expiração".
func TestPrivilegedGrantAuthorizesWithinWindowOnly(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(30 * time.Minute)
	g, _ := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 2, nb, exp)

	// Ainda em requested: não autoriza.
	if g.Authorizes(nb.Add(time.Minute)) {
		t.Fatalf("grant requested não deveria autorizar")
	}

	// Ativo e dentro da janela: autoriza.
	g.Status = GrantActive
	if !g.Authorizes(nb.Add(time.Minute)) {
		t.Fatalf("grant ativo dentro da janela deveria autorizar")
	}
	// Antes do início: não autoriza.
	if g.Authorizes(nb.Add(-time.Minute)) {
		t.Fatalf("antes de NotBefore não deveria autorizar")
	}
	// Após a expiração: NÃO autoriza mesmo com status ainda 'active' (job não
	// materializou a expiração) — a checagem é no momento da decisão.
	if g.Authorizes(exp) {
		t.Fatalf("no instante da expiração não deveria autorizar")
	}
	if g.Authorizes(exp.Add(time.Minute)) {
		t.Fatalf("após a expiração não deveria autorizar mesmo com status active")
	}
	if !g.Expired(exp) {
		t.Fatalf("Expired deveria ser verdadeiro no instante da expiração")
	}
}

// Máquina de estados feliz: requested → (step-up) awaiting_approval → (2
// aprovações distintas) active (cenário "Solicitação completa").
func TestBreakglassStateMachineHappyPath(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	g, _ := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 2, nb, nb.Add(30*time.Minute))

	if err := g.PassStepUp(); err != nil {
		t.Fatalf("PassStepUp: %v", err)
	}
	if g.Status != GrantAwaitingApproval {
		t.Fatalf("após step-up deveria aguardar aprovação, veio %s", g.Status)
	}

	peer1, peer2 := uuid.New(), uuid.New()
	if err := g.Approve(peer1); err != nil {
		t.Fatalf("Approve peer1: %v", err)
	}
	if g.Status != GrantAwaitingApproval {
		t.Fatalf("uma aprovação não deveria ativar")
	}
	if err := g.Approve(peer2); err != nil {
		t.Fatalf("Approve peer2: %v", err)
	}
	if g.Status != GrantActive {
		t.Fatalf("duas aprovações distintas deveriam ativar, veio %s", g.Status)
	}
	if !g.Authorizes(nb.Add(time.Minute)) {
		t.Fatalf("grant ativo e vigente deveria autorizar")
	}
}

// Aprovações duplicadas não contam; transições fora de estado são recusadas.
func TestBreakglassStateMachineGuards(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	g, _ := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 2, nb, nb.Add(30*time.Minute))

	// Aprovar antes do step-up é inválido.
	if err := g.Approve(uuid.New()); !errors.Is(err, ErrGrantTransition) {
		t.Fatalf("aprovar em requested: err = %v", err)
	}
	_ = g.PassStepUp()

	peer := uuid.New()
	_ = g.Approve(peer)
	if err := g.Approve(peer); !errors.Is(err, ErrGrantDuplicateApproval) {
		t.Fatalf("aprovação duplicada: err = %v", err)
	}
	if g.Status != GrantAwaitingApproval {
		t.Fatalf("duplicata não deveria ativar (só 1 aprovador distinto)")
	}
}

// Expiração e revogação: expirar exige janela vencida; revogar exige ativo.
func TestBreakglassExpireAndRevoke(t *testing.T) {
	org, sub := uuid.New(), uuid.New()
	nb := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	exp := nb.Add(30 * time.Minute)

	// Expiração de um grant ativo.
	g, _ := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 1, nb, exp)
	_ = g.PassStepUp()
	_ = g.Approve(uuid.New())
	if g.Status != GrantActive {
		t.Fatalf("pré-condição: deveria estar ativo")
	}
	// Expirar antes da janela vencer é recusado.
	if err := g.Expire(nb.Add(time.Minute)); !errors.Is(err, ErrGrantTransition) {
		t.Fatalf("expirar prematuramente: err = %v", err)
	}
	if err := g.Expire(exp); err != nil {
		t.Fatalf("Expire na janela vencida: %v", err)
	}
	if g.Status != GrantExpired || g.Authorizes(exp) {
		t.Fatalf("expirado não deveria autorizar")
	}

	// Revogar exige ativo.
	g2, _ := NewPrivilegedGrant(org, sub, validTarget(), GrantBreakglass, 1, nb, exp)
	if err := g2.Revoke(); !errors.Is(err, ErrGrantTransition) {
		t.Fatalf("revogar em requested: err = %v", err)
	}
	_ = g2.PassStepUp()
	_ = g2.Approve(uuid.New())
	if err := g2.Revoke(); err != nil {
		t.Fatalf("Revoke ativo: %v", err)
	}
	if g2.Status != GrantRevoked {
		t.Fatalf("status = %s, quero revoked", g2.Status)
	}
}
