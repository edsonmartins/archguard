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
	"time"

	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/adapters/totp"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// totpStepUp raises a live session's assurance to AAL2 after a valid TOTP proof.
// It verifies the code against the identity's vaulted TOTP seed, then loads the
// session, applies domain StepUp and persists it — all in one identity transaction.
type totpStepUp struct {
	svc   *totp.Service
	creds *postgres.CredentialStore
	pool  *pgxpool.Pool
}

// newTOTPStepUp composes the step-up service from the factory. It errors when the
// secret store is unavailable (a conformant profile without OpenBao).
func newTOTPStepUp(f *Factory) (*totpStepUp, error) {
	vault, err := f.SecretStore()
	if err != nil {
		return nil, err
	}
	svc, err := totp.NewService(totpIssuer, vault)
	if err != nil {
		return nil, err
	}
	return &totpStepUp{svc: svc, creds: postgres.NewCredentialStore(f.Pool()), pool: f.Pool()}, nil
}

// StepUpTOTP verifies the code against the caller's TOTP factor and, on success,
// raises the session to AAL2. Distinguishes: no TOTP factor (ErrNoStrongFactor), a
// wrong code (ErrStepUpDenied — a denial), and infrastructure failures (returned
// as-is, fail-closed). The domain StepUp refuses to LOWER assurance and caps at the
// factor's ceiling (TOTP ≤ AAL2), so the elevation is always honest (INV-8).
func (s *totpStepUp) StepUpTOTP(ctx context.Context, identityID, sessionID uuid.UUID, code string, now time.Time) (domain.AAL, error) {
	creds, err := s.creds.ListByIdentity(ctx, identityID)
	if err != nil {
		return "", err
	}
	var totpCred *domain.Credential
	for i := range creds {
		if creds[i].Type == domain.FactorTOTP {
			totpCred = &creds[i]
			break
		}
	}
	if totpCred == nil {
		return "", apihttp.ErrNoStrongFactor
	}

	ok, err := s.svc.Verify(ctx, *totpCred, code)
	if err != nil {
		return "", err // vault failure — fail-closed
	}
	if !ok {
		return "", apihttp.ErrStepUpDenied // wrong code — a denial, not an error
	}

	scope, err := domain.NewIdentityScope(identityID)
	if err != nil {
		return "", err
	}
	var newAAL domain.AAL
	err = postgres.NewIdentityRepository(s.pool, scope).WithIdentityTx(ctx, func(itx *postgres.IdentityTx) error {
		store := postgres.NewIdentitySessionStore(itx)
		session, gerr := store.Get(ctx, sessionID)
		if gerr != nil {
			return gerr
		}
		if serr := session.StepUp(now, domain.AAL2, []domain.FactorType{domain.FactorTOTP}); serr != nil {
			return serr
		}
		if serr := store.SaveStepUp(ctx, session); serr != nil {
			return serr
		}
		newAAL = session.ProvenAAL
		return nil
	})
	if err != nil {
		return "", err
	}
	return newAAL, nil
}
