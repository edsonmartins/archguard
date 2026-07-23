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

func delegationGuard(t *testing.T) *DelegationScopeGuard {
	t.Helper()
	cat, err := BuildOperationCatalog()
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	return NewDelegationScopeGuard(cat)
}

// Uma sessão de delegação pode ler/dar suporte, mas NÃO administra, não toca
// segredos e não aprova (cenário "Tentativa de escalada" e "Tentativa de
// aprovação").
func TestDelegationScopeGuard(t *testing.T) {
	g := delegationGuard(t)

	// Operações de suporte permitidas sob delegação.
	for _, op := range []string{"profile.read", "session.list", string(ActionMembershipAccept)} {
		if err := g.Authorize(op, true); err != nil {
			t.Fatalf("delegação deveria permitir %q: %v", op, err)
		}
	}

	// Escalada administrativa negada.
	if err := g.Authorize(string(ActionAdminMutation), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("mutação administrativa sob delegação: err = %v, quero ErrDelegationScopeExceeded", err)
	}
	// Segredo/cofre negado.
	if err := g.Authorize(string(ActionKeyRotate), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("rotação de chave sob delegação: err = %v", err)
	}
	// Aprovação de break-glass negada.
	if err := g.Authorize(string(ActionBreakglassApprove), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("aprovação de break-glass sob delegação: err = %v", err)
	}
	// Aprovação de recuperação negada.
	if err := g.Authorize(string(ActionRecoveryApprove), true); !errors.Is(err, ErrDelegationScopeExceeded) {
		t.Fatalf("aprovação de recuperação sob delegação: err = %v", err)
	}
}

// Fora de delegação, o guard não interfere (é só um no-op).
func TestDelegationScopeGuardIgnoresOrdinarySessions(t *testing.T) {
	g := delegationGuard(t)
	if err := g.Authorize(string(ActionAdminMutation), false); err != nil {
		t.Fatalf("sessão comum não deveria ser bloqueada por este guard: %v", err)
	}
}

// Fail-closed: operação não classificada sob delegação é recusada.
func TestDelegationScopeGuardFailsClosedOnUnclassified(t *testing.T) {
	g := delegationGuard(t)
	if err := g.Authorize("op.inexistente", true); !errors.Is(err, ErrOperationNotClassified) {
		t.Fatalf("não classificada sob delegação: err = %v, quero ErrOperationNotClassified", err)
	}
}
