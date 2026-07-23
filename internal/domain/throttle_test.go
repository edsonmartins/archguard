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
	"testing"
	"time"
)

// Abaixo do limiar não há bloqueio; a partir do limiar o bloqueio começa e
// cresce a cada falha adicional (progressivo).
func TestThrottleProgressiveLockout(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	var th Throttle

	// Quatro falhas: ainda sem bloqueio.
	for i := 0; i < throttleThreshold-1; i++ {
		th = th.RecordFailure(now)
		if th.Locked(now) {
			t.Fatalf("não deveria bloquear na falha %d", th.Failures)
		}
	}

	// A quinta falha (limiar) bloqueia por throttleBase.
	th = th.RecordFailure(now)
	if !th.Locked(now) {
		t.Fatalf("deveria bloquear no limiar")
	}
	if th.Locked(now.Add(throttleBase)) {
		t.Fatalf("bloqueio deveria expirar após throttleBase")
	}
	firstWindow := th.LockedUntil.Sub(now)

	// A sexta falha bloqueia por MAIS tempo (progressivo).
	th6 := th.RecordFailure(now)
	if th6.LockedUntil.Sub(now) <= firstWindow {
		t.Fatalf("a janela de bloqueio deveria crescer: %v <= %v", th6.LockedUntil.Sub(now), firstWindow)
	}
}

// O bloqueio é limitado por throttleMax mesmo com muitas falhas.
func TestThrottleCappedAtMax(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	th := Throttle{Failures: 100}
	next := th.RecordFailure(now)
	if next.LockedUntil.Sub(now) > throttleMax {
		t.Fatalf("bloqueio não deveria exceder throttleMax: %v", next.LockedUntil.Sub(now))
	}
	if next.LockedUntil.Sub(now) != throttleMax {
		t.Fatalf("com muitas falhas o bloqueio deveria ser exatamente throttleMax, veio %v", next.LockedUntil.Sub(now))
	}
}

// Um sucesso zera o estado — sem punir o usuário honesto depois.
func TestThrottleSuccessResets(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	th := Throttle{Failures: 7}.RecordFailure(now)
	if !th.Locked(now) {
		t.Fatalf("pré-condição: deveria estar bloqueado")
	}
	clean := th.RecordSuccess()
	if clean.Failures != 0 || clean.Locked(now) {
		t.Fatalf("sucesso deveria zerar o estado: %+v", clean)
	}
}
