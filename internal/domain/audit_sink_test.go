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
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeSink struct {
	err      error
	recorded int
}

func (f *fakeSink) Record(_ context.Context, _ AuditEventInput) (SealedEvent, error) {
	if f.err != nil {
		return SealedEvent{}, f.err
	}
	f.recorded++
	return SealedEvent{Seq: int64(f.recorded)}, nil
}

func TestRecordOrDenyFailsClosed(t *testing.T) {
	in := AuditEventInput{
		OrganizationID: uuid.New(),
		Action:         ActionPrivilegedSessionOpen,
		Actor:          AuditActor{IdentitySubject: "sub"},
		Outcome:        Allowed,
	}

	// Sink indisponível ⇒ ErrAuditUnavailable (a operação privilegiada nega).
	down := &fakeSink{err: errors.New("banco fora")}
	if _, err := RecordOrDeny(context.Background(), down, in); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("sink indisponível: err = %v, quero ErrAuditUnavailable", err)
	}
	if down.recorded != 0 {
		t.Fatalf("nada deveria ter sido registrado")
	}

	// Sink disponível ⇒ evento selado retornado, sem erro.
	up := &fakeSink{}
	sealed, err := RecordOrDeny(context.Background(), up, in)
	if err != nil {
		t.Fatalf("sink disponível: %v", err)
	}
	if sealed.Seq != 1 || up.recorded != 1 {
		t.Fatalf("registro não ocorreu: %+v", sealed)
	}
}

func TestRequirePrivileged(t *testing.T) {
	// L3 exige o caminho síncrono fail-closed.
	for _, a := range []Action{ActionPrivilegedSessionOpen, ActionBreakglassRequest, ActionKeyRotate, ActionAuditExport, ActionIdentityDeprovision} {
		if !a.RequirePrivileged() {
			t.Errorf("ação L3 %q deveria exigir gravação privilegiada", a)
		}
	}
	// L1/L2 podem ir para a fila assíncrona (T-009).
	for _, a := range []Action{ActionAuthLogin, ActionTenantSwitch, ActionMembershipInvite, ActionAdminMutation} {
		if a.RequirePrivileged() {
			t.Errorf("ação não-L3 %q não deveria exigir gravação privilegiada", a)
		}
	}
}
