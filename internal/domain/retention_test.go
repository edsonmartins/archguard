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
)

func TestNewRetentionPolicy(t *testing.T) {
	if _, err := NewRetentionPolicy(0); !errors.Is(err, ErrInvalidRetention) {
		t.Fatalf("período zero deveria ser inválido")
	}
	if _, err := NewRetentionPolicy(-time.Hour); !errors.Is(err, ErrInvalidRetention) {
		t.Fatalf("período negativo deveria ser inválido")
	}
	if _, err := NewRetentionPolicy(365 * 24 * time.Hour); err != nil {
		t.Fatalf("período positivo deveria ser válido: %v", err)
	}
}

// Uma partição totalmente além do prazo é elegível a arquivamento; uma dentro da
// janela (mesmo parcialmente) é mantida online.
func TestRetentionPastRetention(t *testing.T) {
	p, _ := NewRetentionPolicy(365 * 24 * time.Hour)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	// Partição que termina há 2 anos: totalmente expirada => arquivar.
	oldTo := now.Add(-2 * 365 * 24 * time.Hour)
	if !p.PastRetention(oldTo, now) {
		t.Fatalf("partição de 2 anos atrás deveria estar além da retenção")
	}
	// Partição que termina ontem: dentro da janela => manter.
	recentTo := now.Add(-24 * time.Hour)
	if p.PastRetention(recentTo, now) {
		t.Fatalf("partição recente deveria ser mantida online")
	}
	// Exatamente no cutoff (to == cutoff): não é APÓS o cutoff => elegível.
	if !p.PastRetention(p.Cutoff(now), now) {
		t.Fatalf("partição terminando exatamente no cutoff deveria ser elegível")
	}
}
