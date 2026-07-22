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

// Package totp adapts the pquerna/otp library (already in the tree — Casdoor
// uses it) to the ArchGuard factor model (pacote 005 T-003). TOTP is a FALLBACK
// second factor: it is capped at AAL2 by construction (domain.MaxAAL), so it can
// never satisfy an L3 step-up — that gate is WebAuthn only (ADR-0010). The TOTP
// seed is a REVERSIBLE secret: it is custodied in the vault (domain.SecretStore)
// and the credential holds only a reference (INV-7 / I-4.3). Enrollment is a two
// step ceremony — the seed reaches the vault ONLY after the user proves posession
// with a valid code, so an unconfirmed seed is never persisted.
package totp

import (
	"context"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTP parameters. SHA-1 / 6 digits / 30s is the profile every authenticator app
// (Google Authenticator, Aegis, 1Password…) interoperates with. skew=1 tolerates
// one period of clock drift on either side — no wider, to bound replay.
const (
	periodSeconds = 30
	digits        = otp.DigitsSix
	algorithm     = otp.AlgorithmSHA1
	skew          = 1
)

// Service enrolls and verifies TOTP factors against a vault. The vault custodies
// the seed; this service never persists it.
type Service struct {
	issuer string
	vault  domain.SecretStore
}

// NewService builds the TOTP service. issuer is the non-personal label shown in
// the authenticator app (the product/tenant name); vault custodies the seeds.
func NewService(issuer string, vault domain.SecretStore) (*Service, error) {
	if issuer == "" {
		return nil, fmt.Errorf("totp: issuer vazio")
	}
	if vault == nil {
		return nil, fmt.Errorf("totp: vault nulo")
	}
	return &Service{issuer: issuer, vault: vault}, nil
}

// Enrollment is the in-progress, server-side state of a TOTP enrollment. It holds
// the raw seed EPHEMERALLY (in memory only) until the user confirms possession —
// it is never persisted, logged or traced. ProvisioningURI carries the seed and
// is shown ONCE to the enrolling user (over TLS) so they can scan the QR; it is
// the one place the seed is legitimately disclosed to its owner.
type Enrollment struct {
	IdentityID      uuid.UUID
	ProvisioningURI string
	// secret is the base32 TOTP seed, kept unexported so it does not leak through
	// reflection-based logging of the struct.
	secret string
}

// BeginEnrollment generates a fresh TOTP seed for the identity and returns the
// enrollment state plus the provisioning URI. It does NOT touch the vault or the
// database — the seed stays in memory until FinishEnrollment confirms it, so a
// seed the user never configures is simply discarded. accountName is a
// non-personal per-identity label (e.g. the opaque subject) for the app UI.
func (s *Service) BeginEnrollment(identityID uuid.UUID, accountName string) (*Enrollment, error) {
	if identityID == uuid.Nil {
		return nil, fmt.Errorf("totp: identidade nula")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: accountName,
		Period:      periodSeconds,
		Digits:      digits,
		Algorithm:   algorithm,
	})
	if err != nil {
		return nil, fmt.Errorf("totp: geração de semente falhou: %w", err)
	}
	return &Enrollment{
		IdentityID:      identityID,
		ProvisioningURI: key.URL(),
		secret:          key.Secret(),
	}, nil
}

// FinishEnrollment confirms possession by validating a code against the pending
// seed, then — and only then — writes the seed to the vault and returns the
// factor as a domain.Credential (SecretRef only, AAL2, INV-7 shape). A wrong code
// returns an error and nothing is persisted. The vault write happens here,
// OUTSIDE any database transaction (RFC-0004 §4); the caller persists the
// returned credential in its own transaction and, if that transaction fails,
// compensates with vault Delete on the returned SecretRef.
func (s *Service) FinishEnrollment(ctx context.Context, e *Enrollment, code string) (domain.Credential, error) {
	if e == nil || e.secret == "" {
		return domain.Credential{}, fmt.Errorf("totp: enrollment inválido")
	}
	if !s.validate(code, e.secret) {
		return domain.Credential{}, fmt.Errorf("totp: código de confirmação inválido")
	}
	ref, err := s.vault.Put(ctx, []byte(e.secret))
	if err != nil {
		return domain.Credential{}, fmt.Errorf("totp: custódia da semente falhou: %w", err)
	}
	cred, err := domain.NewTOTPCredential(e.IdentityID, ref)
	if err != nil {
		// The seed was vaulted but no credential will reference it — compensate so
		// it does not outlive its intended row (INV-7 leaves no orphan secret).
		_ = s.vault.Delete(ctx, ref)
		return domain.Credential{}, err
	}
	cred.Label = "TOTP"
	return cred, nil
}

// Verify checks a code against the credential's vaulted seed — the path used at
// login and at an L2 step-up. It requires a TOTP credential and resolves the seed
// from the vault for the duration of the check only; the seed is never returned
// to or persisted by the caller. A vault failure is an ERROR (fail-closed,
// INV-6), distinct from a wrong code (a denial: ok=false, err=nil).
func (s *Service) Verify(ctx context.Context, cred domain.Credential, code string) (ok bool, err error) {
	if cred.Type != domain.FactorTOTP {
		return false, fmt.Errorf("totp: credencial não é TOTP: %s", cred.Type)
	}
	if cred.SecretRef == "" {
		return false, fmt.Errorf("totp: credencial sem referência de semente")
	}
	seed, err := s.vault.Get(ctx, cred.SecretRef)
	if err != nil {
		return false, fmt.Errorf("totp: resolução da semente falhou: %w", err)
	}
	return s.validate(code, string(seed)), nil
}

// validate runs the RFC-6238 check with the service's fixed profile.
func (s *Service) validate(code, secret string) bool {
	ok, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    periodSeconds,
		Skew:      skew,
		Digits:    digits,
		Algorithm: algorithm,
	})
	if err != nil {
		return false
	}
	return ok
}
