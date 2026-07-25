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
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/deploy"
)

func TestFactoryProfileAndPool(t *testing.T) {
	f := NewFactory(deploy.Dev, nil)
	if f.Profile() != deploy.Dev {
		t.Fatalf("Profile() = %v, want %v", f.Profile(), deploy.Dev)
	}
	if f.Pool() != nil {
		t.Fatalf("Pool() should return the pool it was built with (nil here)")
	}
}

func TestCustodyAvailableInDev(t *testing.T) {
	f := NewFactory(deploy.Dev, nil)
	if !f.CustodyAvailable() {
		t.Fatalf("dev profile should have custody available (local/provisional)")
	}
	if err := f.RequireCustody(); err != nil {
		t.Fatalf("RequireCustody in dev should be nil, got %v", err)
	}
}

// TestCustodyFailsClosedInConformant is the INV-6/INV-7 guard: a conformant
// profile must refuse custody until OpenBao is wired, never downgrade to dev
// custody. Covers spec scenario "Adapter de desenvolvimento em perfil conforme".
func TestCustodyFailsClosedInConformant(t *testing.T) {
	for _, p := range []deploy.Profile{deploy.Pilot, deploy.Production} {
		f := NewFactory(p, nil)
		if f.CustodyAvailable() {
			t.Fatalf("profile %v must NOT report custody available (OpenBao not wired)", p)
		}
		err := f.RequireCustody()
		if err == nil {
			t.Fatalf("profile %v: RequireCustody should fail closed", p)
		}
		if !errors.Is(err, ErrCustodyBackendUnavailable) {
			t.Fatalf("profile %v: want ErrCustodyBackendUnavailable, got %v", p, err)
		}
	}
}
