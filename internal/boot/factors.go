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
	"sync"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/adapters/totp"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
)

// totpIssuer is the non-personal label shown in the authenticator app.
const totpIssuer = "ArchGuard"

// totpEnroller drives the two-step TOTP enrollment ceremony. The pending seed is
// held in memory between begin and verify — the totp.Enrollment keeps it
// unexported, so it cannot round-trip through the client. This is dev-scoped
// (single instance, ephemeral, lost on restart); a shared short-TTL store is
// required for production/replicas. The seed is never persisted or logged here.
type totpEnroller struct {
	svc   *totp.Service
	creds *postgres.CredentialStore
	vault domain.SecretStore

	mu      sync.Mutex
	pending map[uuid.UUID]*totp.Enrollment
}

// newTOTPEnroller composes the enroller from the factory. It errors when the
// secret store is unavailable (a conformant profile without OpenBao) — the caller
// then mounts the enrollment endpoints fail-closed.
func newTOTPEnroller(f *Factory) (*totpEnroller, error) {
	vault, err := f.SecretStore()
	if err != nil {
		return nil, err
	}
	svc, err := totp.NewService(totpIssuer, vault)
	if err != nil {
		return nil, err
	}
	return &totpEnroller{
		svc:     svc,
		creds:   postgres.NewCredentialStore(f.Pool()),
		vault:   vault,
		pending: map[uuid.UUID]*totp.Enrollment{},
	}, nil
}

// BeginTOTP generates a fresh seed and returns the provisioning URI (which carries
// the seed for the QR). The pending enrollment is held server-side until verify.
func (e *totpEnroller) BeginTOTP(_ context.Context, identityID uuid.UUID) (string, error) {
	enr, err := e.svc.BeginEnrollment(identityID, identityID.String())
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	e.pending[identityID] = enr
	e.mu.Unlock()
	return enr.ProvisioningURI, nil
}

// FinishTOTP confirms possession with a code, vaults the seed and persists the
// credential. If persistence fails after the seed was vaulted, it compensates by
// deleting the vaulted seed (the adapter's contract — no orphan secret, INV-7).
func (e *totpEnroller) FinishTOTP(ctx context.Context, identityID uuid.UUID, code string) error {
	e.mu.Lock()
	enr := e.pending[identityID]
	e.mu.Unlock()
	if enr == nil {
		return apihttp.ErrNoPendingEnrollment
	}

	cred, err := e.svc.FinishEnrollment(ctx, enr, code)
	if err != nil {
		return err // wrong confirmation code, or a vault failure
	}
	if err := e.creds.Create(ctx, cred); err != nil {
		_ = e.vault.Delete(ctx, cred.SecretRef) // compensate: no orphan seed
		return err
	}

	e.mu.Lock()
	delete(e.pending, identityID)
	e.mu.Unlock()
	return nil
}
