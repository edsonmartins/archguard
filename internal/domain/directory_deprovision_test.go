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

import "testing"

// A desativação no diretório suspende SÓ um membership ativo; qualquer outro
// estado é no-op (idempotente). Uma entrada ativa nunca dispara suspensão.
func TestRequiresSuspension(t *testing.T) {
	inactive := DirectorySyncRecord{Active: false}
	active := DirectorySyncRecord{Active: true}

	if !inactive.RequiresSuspension(MembershipActive) {
		t.Fatalf("desativação no diretório sobre membership ativo deveria suspender")
	}
	if active.RequiresSuspension(MembershipActive) {
		t.Fatalf("entrada ativa não deveria suspender")
	}
	for _, st := range []MembershipStatus{MembershipSuspended, MembershipRevoked, MembershipInvited} {
		if inactive.RequiresSuspension(st) {
			t.Fatalf("membership em %s não deveria disparar suspensão (idempotente)", st)
		}
	}
}
