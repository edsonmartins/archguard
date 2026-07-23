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

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// Falhas repetidas persistem e, ao passar o limiar, a identidade fica bloqueada;
// um sucesso zera o estado (cenário "Tentativas repetidas").
func TestThrottleStoreProgressiveLockoutPersists(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "throttle")

	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := NewIdentityRepository(pool, fx.scopeIdn)

	recordFailure := func() domain.Throttle {
		var out domain.Throttle
		if err := repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
			s := NewThrottleStore(itx)
			th, e := s.Get(ctx, fx.identity.ID)
			if e != nil {
				return e
			}
			out = th.RecordFailure(now)
			return s.Save(ctx, fx.identity.ID, out)
		}); err != nil {
			t.Fatalf("recordFailure: %v", err)
		}
		return out
	}

	// Abaixo do limiar: sem bloqueio persistido.
	var th domain.Throttle
	for i := 0; i < 4; i++ {
		th = recordFailure()
	}
	if th.Locked(now) {
		t.Fatalf("4 falhas não deveriam bloquear")
	}

	// A quinta falha bloqueia; releitura confirma persistência.
	recordFailure()
	var reread domain.Throttle
	if err := repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
		var e error
		reread, e = NewThrottleStore(itx).Get(ctx, fx.identity.ID)
		return e
	}); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reread.Failures != 5 || !reread.Locked(now) {
		t.Fatalf("após 5 falhas deveria estar bloqueado: %+v", reread)
	}

	// Sucesso zera.
	if err := repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
		s := NewThrottleStore(itx)
		th, e := s.Get(ctx, fx.identity.ID)
		if e != nil {
			return e
		}
		return s.Save(ctx, fx.identity.ID, th.RecordSuccess())
	}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var cleared domain.Throttle
	_ = repo.WithIdentityTx(ctx, func(itx *IdentityTx) error {
		cleared, _ = NewThrottleStore(itx).Get(ctx, fx.identity.ID)
		return nil
	})
	if cleared.Failures != 0 || cleared.Locked(now) {
		t.Fatalf("sucesso deveria zerar o throttle persistido: %+v", cleared)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM auth_throttle WHERE identity_id = $1", fx.identity.ID.String())
	})
}

// Barreira 1: o store escopado a uma identidade recusa mexer no throttle de
// outra.
func TestThrottleStoreRejectsCrossIdentity(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	fx := makeSessionFixture(t, pool, "throttlext")

	err := NewIdentityRepository(pool, fx.scopeIdn).WithIdentityTx(ctx, func(itx *IdentityTx) error {
		s := NewThrottleStore(itx)
		if _, e := s.Get(ctx, uuid.New()); e == nil {
			t.Fatalf("Get de outra identidade deveria ser recusado")
		}
		if e := s.Save(ctx, uuid.New(), domain.Throttle{Failures: 1}); e == nil {
			t.Fatalf("Save de outra identidade deveria ser recusado")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
}
