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

package boot

import (
	"context"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// TestProvenAALFromMethods pins the conservative AAL derivation: it reflects the
// strongest factor proven and never over-claims (over-claiming would violate INV-8).
func TestProvenAALFromMethods(t *testing.T) {
	cases := []struct {
		name    string
		methods []domain.FactorType
		want    domain.AAL
	}{
		{"empty is AAL1", nil, domain.AAL1},
		{"password only", []domain.FactorType{domain.FactorPassword}, domain.AAL1},
		{"password + TOTP", []domain.FactorType{domain.FactorPassword, domain.FactorTOTP}, domain.AAL2},
		{"webauthn", []domain.FactorType{domain.FactorWebAuthn}, domain.AAL3},
		{"password + webauthn takes the strongest", []domain.FactorType{domain.FactorPassword, domain.FactorWebAuthn}, domain.AAL3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provenAALFromMethods(tc.methods); got != tc.want {
				t.Fatalf("provenAALFromMethods(%v) = %v, want %v", tc.methods, got, tc.want)
			}
		})
	}
}

// TestBridgeLoginNoFactoryFailsClosed: without an active factory (InitFactory not
// run), the bridge establishes nothing and returns no error — the legacy login
// proceeds and the domain API stays fail-closed.
func TestBridgeLoginNoFactoryFailsClosed(t *testing.T) {
	factoryMu.Lock()
	activeFactory = nil
	factoryMu.Unlock()

	_, _, established, err := BridgeLogin(context.Background(), "x@example.com", []domain.FactorType{domain.FactorPassword}, time.Time{})
	if err != nil {
		t.Fatalf("BridgeLogin with no factory should not error, got %v", err)
	}
	if established {
		t.Fatalf("BridgeLogin with no factory must not establish a session")
	}
}
