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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/adapters/keycustodian"
	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// custodyKeyName is the sealed-keystore entry under which the dev key custodian's
// deployment key lives. Generated once and reused so email_hash stays stable
// across restarts (dedup depends on a stable HMAC key).
const custodyKeyName = "archguard-custody-deployment-key"

// Factory is the single place where the deployment profile decides which adapter
// backs each capability — the selection that was previously dispersed across the
// provisional adapters (each consulting deploy.Active() on its own).
//
// Profile-invariant capabilities (the postgres stores) use the runtime pool
// directly in every profile. Profile-varying custody capabilities (KeyCustodian,
// token signer) are gated here, fail-closed (INV-6/INV-7): a conformant profile
// requires the real OpenBao-backed backend, which is not wired in the main repo
// yet (archguard-devops / pacote 010 T-010). Until it is, custody is reported
// unavailable for conformant profiles — never silently downgraded to the dev
// custodian (INV-7: no dev custody in production).
//
// The gate is consulted lazily, per capability, by the code that needs it (the
// admin seed and the custody-dependent handlers). It is NOT a boot-time panic: a
// conformant instance still boots and serves the non-custody endpoints; only
// custody-dependent operations refuse.
type Factory struct {
	profile  deploy.Profile
	pool     *pgxpool.Pool
	keystore *keystore.SealedKeystore // dev key material store; nil outside dev

	custodyOnce sync.Once
	custody     domain.KeyCustodian
	custodyErr  error
}

// NewFactory builds a Factory for the active deployment profile, runtime pool and
// (dev only) sealed keystore. The keystore may be nil outside dev.
func NewFactory(profile deploy.Profile, pool *pgxpool.Pool, ks *keystore.SealedKeystore) *Factory {
	return &Factory{profile: profile, pool: pool, keystore: ks}
}

// Pool returns the runtime pool the composition root passes to the stores. It is
// the same backend (PostgreSQL, ADR-0009) in every profile.
func (f *Factory) Pool() *pgxpool.Pool { return f.pool }

// Profile returns the active deployment profile.
func (f *Factory) Profile() deploy.Profile { return f.profile }

// ErrCustodyBackendUnavailable is returned (fail-closed) when the custody backend
// the active profile requires is not available to the composition root. Today
// this is every conformant profile: the OpenBao-backed custodian is not wired in
// the main repo (it is archguard-devops / pacote 010 T-010). A custody-dependent
// path consulted under a conformant profile MUST refuse with this error, not fall
// back to the dev/provisional custodian (INV-7, ADR-0017).
var ErrCustodyBackendUnavailable = errors.New(
	"boot: custody backend indisponível para o perfil ativo — o OpenBao não está ligado no repo principal (ver docs/DEVOPS-HANDOFF.md); operação de custódia negada (INV-6/INV-7)")

// CustodyAvailable reports whether the composition root can serve custody-
// dependent capabilities under the active profile. Dev uses the local/provisional
// custodian, so it is available; conformant profiles require OpenBao, which is not
// wired yet, so they are not.
func (f *Factory) CustodyAvailable() bool { return f.profile.IsDev() }

// RequireCustody returns nil when custody is available under the active profile,
// or ErrCustodyBackendUnavailable otherwise. Custody-dependent code calls this
// first and refuses (fail-closed) rather than degrade to dev custody in production.
func (f *Factory) RequireCustody() error {
	if f.CustodyAvailable() {
		return nil
	}
	return ErrCustodyBackendUnavailable
}

// KeyCustodian returns the key custodian for the active profile, building it once.
//
// Dev: an in-process Provisional custodian keyed by a 256-bit deployment key
// generated on first use and persisted in the sealed keystore, so the e-mail HMAC
// (and thus dedup) is stable across restarts. The Provisional custodian is a
// development stand-in, never production (ADR-0012).
//
// Conformant: ErrCustodyBackendUnavailable — the OpenBao-backed custodian is not
// wired here; the caller must refuse the custody-dependent operation (fail-closed).
func (f *Factory) KeyCustodian() (domain.KeyCustodian, error) {
	f.custodyOnce.Do(func() {
		if !f.profile.IsDev() {
			f.custodyErr = ErrCustodyBackendUnavailable
			return
		}
		if f.keystore == nil {
			f.custodyErr = fmt.Errorf("boot: keystore de dev não aberto; não é possível derivar a chave de custódia")
			return
		}
		key, err := devCustodyKey(f.keystore)
		if err != nil {
			f.custodyErr = err
			return
		}
		cust, err := keycustodian.NewProvisional(key)
		if err != nil {
			f.custodyErr = err
			return
		}
		f.custody = cust
	})
	return f.custody, f.custodyErr
}

// devCustodyKey returns the 256-bit dev custody deployment key, generating and
// persisting it in the sealed keystore on first use (stored hex-encoded) so it is
// stable across restarts. Regenerating it would change every email_hash and break
// dedup, so an existing entry is always reused.
func devCustodyKey(ks *keystore.SealedKeystore) ([]byte, error) {
	if hexKey, ok := ks.Get(custodyKeyName); ok {
		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("boot: chave de custódia dev corrompida no keystore: %w", err)
		}
		if len(key) < 32 {
			return nil, fmt.Errorf("boot: chave de custódia dev no keystore com menos de 256 bits")
		}
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("boot: geração da chave de custódia dev falhou: %w", err)
	}
	if err := ks.Put(custodyKeyName, hex.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("boot: persistência da chave de custódia dev falhou: %w", err)
	}
	return key, nil
}

// The active factory is a boot singleton, built once (InitFactory) and consulted
// by the admin seed and the custody-dependent handlers.
var (
	factoryMu     sync.Mutex
	activeFactory *Factory
)

// InitFactory builds the adapter factory for the active profile and stores it.
// Must run at boot after InitPool.
func InitFactory(profile deploy.Profile, pool *pgxpool.Pool, ks *keystore.SealedKeystore) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	activeFactory = NewFactory(profile, pool, ks)
}

// ActiveFactory returns the boot factory, or nil if InitFactory has not run.
func ActiveFactory() *Factory {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	return activeFactory
}
