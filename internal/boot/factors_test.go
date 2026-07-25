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
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/secretstore"
	"github.com/casdoor/casdoor/internal/adapters/totp"
	"github.com/google/uuid"
)

// TestTOTPBeginGeneratesAndHoldsPending exercises the enrollment begin without a
// database: it generates a seed, returns a provisioning URI and holds the pending
// enrollment server-side (the seed cannot round-trip through the client).
func TestTOTPBeginGeneratesAndHoldsPending(t *testing.T) {
	vault := secretstore.NewProvisional(openTempKeystore(t))
	svc, err := totp.NewService("ArchGuard", vault)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	enroller := &totpEnroller{svc: svc, vault: vault, pending: map[uuid.UUID]*totp.Enrollment{}}

	id := uuid.New()
	uri, err := enroller.BeginTOTP(context.Background(), id)
	if err != nil {
		t.Fatalf("BeginTOTP: %v", err)
	}
	if !strings.HasPrefix(uri, "otpauth://") {
		t.Fatalf("provisioning uri = %q, want otpauth://…", uri)
	}
	enroller.mu.Lock()
	_, held := enroller.pending[id]
	enroller.mu.Unlock()
	if !held {
		t.Fatalf("pending enrollment must be held server-side between begin and verify")
	}
}
