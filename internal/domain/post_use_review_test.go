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

func terminatedBreakglass(t *testing.T, status GrantStatus) PrivilegedGrant {
	t.Helper()
	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	g, err := NewBreakglassRequest(uuid.New(), uuid.New(), GrantTarget{Type: "asset", ID: "x", Scope: "admin"}, 1, "inc", "INC-1", nb, nb.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewBreakglassRequest: %v", err)
	}
	g.Status = status
	return g
}

// Só break-glass terminado (expired/revoked) requer revisão pós-uso.
func TestGrantNeedsReview(t *testing.T) {
	if !terminatedBreakglass(t, GrantExpired).NeedsReview() {
		t.Fatalf("break-glass expirado deveria requerer revisão")
	}
	if !terminatedBreakglass(t, GrantRevoked).NeedsReview() {
		t.Fatalf("break-glass revogado deveria requerer revisão")
	}
	// Denied/rejected (nunca ativou) não requer.
	if terminatedBreakglass(t, GrantDenied).NeedsReview() {
		t.Fatalf("break-glass negado não deveria requerer revisão")
	}
	// Normal não requer.
	nb := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	normal, _ := NewPrivilegedGrant(uuid.New(), uuid.New(), GrantTarget{Type: "a", ID: "b", Scope: "c"}, GrantNormal, 1, nb, nb.Add(time.Hour))
	normal.Status = GrantExpired
	if normal.NeedsReview() {
		t.Fatalf("concessão normal não deveria requerer revisão pós-uso")
	}
}

func TestNewPostUseReview(t *testing.T) {
	g := terminatedBreakglass(t, GrantExpired)
	reviewer := uuid.New()

	// Parecer obrigatório.
	if _, err := NewPostUseReview(g, reviewer, ""); !errors.Is(err, ErrInvalidReview) {
		t.Fatalf("sem parecer: err = %v", err)
	}
	// Concessão que não requer revisão é recusada.
	active := terminatedBreakglass(t, GrantActive)
	if _, err := NewPostUseReview(active, reviewer, "ok"); !errors.Is(err, ErrInvalidReview) {
		t.Fatalf("concessão ativa não requer revisão: err = %v", err)
	}

	r, err := NewPostUseReview(g, reviewer, "acesso legítimo, incidente confirmado")
	if err != nil {
		t.Fatalf("NewPostUseReview: %v", err)
	}
	if r.GrantID != g.ID || r.ReviewerMembershipID != reviewer || r.Notes == "" {
		t.Fatalf("revisão inesperada: %+v", r)
	}
}
