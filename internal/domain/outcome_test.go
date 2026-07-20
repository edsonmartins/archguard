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

func TestOnlyAllowedPermits(t *testing.T) {
	// INV-6 fail-closed: neither a denial nor a control failure may permit.
	if !Allowed.Permitted() {
		t.Error("Allowed deve permitir")
	}
	if Denied.Permitted() {
		t.Error("Denied não pode permitir")
	}
	if Failed.Permitted() {
		t.Error("Failed não pode permitir (fail-closed, INV-6)")
	}
}

func TestOutcomeString(t *testing.T) {
	cases := map[Outcome]string{Allowed: "allowed", Denied: "denied", Failed: "failed", Outcome(99): "unknown"}
	for o, want := range cases {
		if got := o.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, quer %q", int(o), got, want)
		}
	}
}
