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
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeTx embeds pgx.Tx (nil) and records Commit/Rollback. Only those two are
// exercised; any other call would panic, which is fine for these tests.
type fakeTx struct {
	pgx.Tx
	committed  bool
	rolledback bool
}

func (f *fakeTx) Commit(context.Context) error   { f.committed = true; return nil }
func (f *fakeTx) Rollback(context.Context) error { f.rolledback = true; return nil }

type fakeBeginner struct {
	tx     *fakeTx
	begErr error
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	if b.begErr != nil {
		return nil, b.begErr
	}
	return b.tx, nil
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	err := WithTx(context.Background(), &fakeBeginner{tx: tx}, func(pgx.Tx) error { return nil })
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !tx.committed || tx.rolledback {
		t.Fatalf("deveria commitar e não rollback: committed=%v rolledback=%v", tx.committed, tx.rolledback)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	tx := &fakeTx{}
	want := errors.New("boom")
	err := WithTx(context.Background(), &fakeBeginner{tx: tx}, func(pgx.Tx) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("erro esperado %v, veio %v", want, err)
	}
	if tx.committed || !tx.rolledback {
		t.Fatalf("deveria rollback e não commitar: committed=%v rolledback=%v", tx.committed, tx.rolledback)
	}
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	tx := &fakeTx{}
	defer func() {
		if recover() == nil {
			t.Fatal("panic deveria propagar")
		}
		if !tx.rolledback || tx.committed {
			t.Fatalf("panic deveria rollback: committed=%v rolledback=%v", tx.committed, tx.rolledback)
		}
	}()
	_ = WithTx(context.Background(), &fakeBeginner{tx: tx}, func(pgx.Tx) error { panic("boom") })
}

func TestWithTxReturnsBeginError(t *testing.T) {
	want := errors.New("no conn")
	err := WithTx(context.Background(), &fakeBeginner{begErr: want}, func(pgx.Tx) error { return nil })
	if !errors.Is(err, want) {
		t.Fatalf("erro de Begin deveria propagar: %v", err)
	}
}
