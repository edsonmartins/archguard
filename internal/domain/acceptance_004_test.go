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

// Testes de ACEITAÇÃO dos cenários nomeados do pacote 004 (T-018/019/020),
// escritos contra o catálogo canônico de operações e os tipos de domínio — a
// mesma classificação/regras que a API usa em produção.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// T-018 — "delegação não escala privilégio nem aprova solicitações": uma sessão
// de delegação não alcança operação administrativa nem aprovação de break-glass,
// mas alcança as operações de suporte.
func TestAcceptanceDelegationDoesNotEscalateNorApprove(t *testing.T) {
	cat, err := BuildOperationCatalog()
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	g := NewDelegationScopeGuard(cat)

	// Não escala privilégio (mutação administrativa negada).
	if err := g.Authorize(string(ActionAdminMutation), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("delegação não deveria escalar para admin: %v", err)
	}
	// Não aprova break-glass.
	if err := g.Authorize(string(ActionBreakglassApprove), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("delegação não deveria aprovar break-glass: %v", err)
	}
	// Não usa/abre concessão privilegiada (não escala).
	if err := g.Authorize(string(ActionPrivilegedGrantUse), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("delegação não deveria usar concessão privilegiada: %v", err)
	}
	// Mas alcança suporte.
	if err := g.Authorize("profile.read", true); err != nil {
		t.Fatalf("delegação deveria alcançar operação de suporte: %v", err)
	}
}

// T-019 — "break-glass sem canal de notificação é negado": sem canal disponível
// a solicitação é recusada e nada é criado.
func TestAcceptanceBreakglassDeniedWithoutChannel(t *testing.T) {
	r := NewBreakglassRequester(&acceptanceNotifier{available: false})
	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	_, err := r.Request(context.Background(), uuid.New(), uuid.New(),
		GrantTarget{Type: "asset", ID: "db", Scope: "admin"},
		BreakglassPolicy{RequiredApprovals: 2}, "incidente", "INC-1", nb, nb.Add(30*time.Minute))
	if !errors.Is(err, ErrNoNotificationChannel) {
		t.Fatalf("sem canal: err = %v, quero ErrNoNotificationChannel", err)
	}
}

// T-020 — "concessão expirada não autoriza acesso mesmo com token válido em
// mãos": a autoridade é avaliada no momento da decisão pela janela, não pelo
// token.
func TestAcceptanceExpiredGrantDeniesEvenWithValidToken(t *testing.T) {
	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	exp := nb.Add(30 * time.Minute)
	g, err := NewBreakglassRequest(uuid.New(), uuid.New(),
		GrantTarget{Type: "asset", ID: "db", Scope: "admin"}, 1, "incidente", "INC-1", nb, exp)
	if err != nil {
		t.Fatalf("NewBreakglassRequest: %v", err)
	}
	_ = g.PassStepUp(AAL3, true)
	_ = g.Approve(uuid.New())
	if !g.Authorizes(nb.Add(time.Minute)) {
		t.Fatalf("pré-condição: concessão ativa deveria autorizar dentro da janela")
	}

	// O status permanece 'active' (o job de expiração ainda não rodou), mas a
	// janela venceu: um token emitido sob a concessão, apresentado após a
	// expiração, NÃO autoriza — a decisão é pela janela.
	if g.Status != GrantActive {
		t.Fatalf("o teste pressupõe status ainda active (expiração não materializada)")
	}
	if g.Authorizes(exp.Add(time.Minute)) {
		t.Fatalf("concessão fora da janela não deveria autorizar mesmo com status active e token válido")
	}
}

type acceptanceNotifier struct{ available bool }

func (n *acceptanceNotifier) Notify(context.Context, Notification) error { return nil }
func (n *acceptanceNotifier) Available(context.Context, string) bool     { return n.available }
