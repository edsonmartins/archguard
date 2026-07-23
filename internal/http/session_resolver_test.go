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

package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeBinding struct {
	id, sid uuid.UUID
	ok      bool
}

func (b fakeBinding) Bound(*http.Request) (uuid.UUID, uuid.UUID, bool) { return b.id, b.sid, b.ok }

type fakeLoader struct {
	session domain.AuthSession
	err     error
}

func (l fakeLoader) LoadSession(context.Context, uuid.UUID, uuid.UUID) (domain.AuthSession, error) {
	return l.session, l.err
}

func activeAuthSession(t *testing.T) domain.AuthSession {
	t.Helper()
	return *buildSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
}

// Vínculo válido + sessão ativa -> resolve a sessão.
func TestBridgingResolverResolves(t *testing.T) {
	r := NewBridgingResolver(
		fakeBinding{id: uuid.New(), sid: uuid.New(), ok: true},
		fakeLoader{session: activeAuthSession(t)},
	)
	s, ok := r.Session(httptest.NewRequest(http.MethodGet, "/authorize", nil))
	if !ok || s == nil || s.Status != domain.SessionActive {
		t.Fatalf("deveria resolver uma sessão ativa: ok=%v s=%v", ok, s)
	}
}

// Sem vínculo (não logado) -> não resolve.
func TestBridgingResolverNoBinding(t *testing.T) {
	r := NewBridgingResolver(fakeBinding{ok: false}, fakeLoader{session: activeAuthSession(t)})
	if _, ok := r.Session(httptest.NewRequest(http.MethodGet, "/authorize", nil)); ok {
		t.Fatalf("sem vínculo não deveria resolver")
	}
}

// Vínculo obsoleto (sessão não carrega) -> não resolve.
func TestBridgingResolverStaleBinding(t *testing.T) {
	r := NewBridgingResolver(fakeBinding{id: uuid.New(), sid: uuid.New(), ok: true},
		fakeLoader{err: errors.New("não encontrada")})
	if _, ok := r.Session(httptest.NewRequest(http.MethodGet, "/authorize", nil)); ok {
		t.Fatalf("vínculo obsoleto não deveria resolver")
	}
}

// Sessão revogada -> não é contexto de login válido.
func TestBridgingResolverRevokedSession(t *testing.T) {
	revoked := activeAuthSession(t)
	revoked.Revoke()
	r := NewBridgingResolver(fakeBinding{id: uuid.New(), sid: uuid.New(), ok: true},
		fakeLoader{session: revoked})
	if _, ok := r.Session(httptest.NewRequest(http.MethodGet, "/authorize", nil)); ok {
		t.Fatalf("sessão revogada não deveria resolver")
	}
}
