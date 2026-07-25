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

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
// The gate is consulted lazily, per capability, by the handlers that need it
// (T-006/T-007). It is NOT a boot-time panic: a conformant instance still boots
// and serves the non-custody endpoints; only custody-dependent operations refuse.
type Factory struct {
	profile deploy.Profile
	pool    *pgxpool.Pool
}

// NewFactory builds a Factory for the active deployment profile and runtime pool.
func NewFactory(profile deploy.Profile, pool *pgxpool.Pool) *Factory {
	return &Factory{profile: profile, pool: pool}
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
// endpoint consulted under a conformant profile MUST refuse with this error, not
// fall back to the dev/provisional custodian (INV-7, ADR-0017).
var ErrCustodyBackendUnavailable = errors.New(
	"boot: custody backend indisponível para o perfil ativo — o OpenBao não está ligado no repo principal (ver docs/DEVOPS-HANDOFF.md); operação de custódia negada (INV-6/INV-7)")

// CustodyAvailable reports whether the composition root can serve custody-
// dependent capabilities under the active profile. Dev uses the local/provisional
// custodian, so it is available; conformant profiles require OpenBao, which is not
// wired yet, so they are not.
func (f *Factory) CustodyAvailable() bool { return f.profile.IsDev() }

// RequireCustody returns nil when custody is available under the active profile,
// or ErrCustodyBackendUnavailable otherwise. Custody-dependent handlers call this
// first and refuse (fail-closed) rather than degrade to dev custody in production.
func (f *Factory) RequireCustody() error {
	if f.CustodyAvailable() {
		return nil
	}
	return ErrCustodyBackendUnavailable
}
