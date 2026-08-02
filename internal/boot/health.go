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

	"github.com/casdoor/casdoor/internal/deploy"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

// healthChecker probes the subsystems the composition root can honestly observe:
// the database (a real ping), custody (profile-based availability) and the
// deployment conformance. Subsystems not yet wired (PDP, audit trail) are reported
// truthfully as they are activated — never faked green.
type healthChecker struct {
	pool    *pgxpool.Pool
	factory *Factory
}

// CheckHealth implements apihttp.HealthChecker.
func (h healthChecker) CheckHealth(ctx context.Context) []apihttp.Subsystem {
	subsystems := []apihttp.Subsystem{
		h.database(ctx),
		h.custody(),
		h.deployment(),
	}
	return subsystems
}

func (h healthChecker) database(ctx context.Context) apihttp.Subsystem {
	if h.pool == nil {
		return apihttp.Subsystem{Name: "database", Status: apihttp.StatusUnavailable, Detail: "pool não inicializado"}
	}
	if err := h.pool.Ping(ctx); err != nil {
		return apihttp.Subsystem{Name: "database", Status: apihttp.StatusUnavailable, Detail: "ping falhou"}
	}
	return apihttp.Subsystem{Name: "database", Status: apihttp.StatusOK}
}

func (h healthChecker) custody() apihttp.Subsystem {
	if h.factory != nil && h.factory.CustodyAvailable() {
		// O detalhe deve refletir o backend REAL, não uma string fixa: em perfil conforme a
		// custódia é o OpenBao (cofre); só o perfil dev usa o keystore local selado. Um produto
		// de segurança não pode mentir sobre onde suas chaves vivem.
		detail := "OpenBao (cofre)"
		if deploy.Active().IsDev() {
			detail = "keystore local selado (dev)"
		}
		return apihttp.Subsystem{Name: "custody", Status: apihttp.StatusOK, Detail: detail}
	}
	return apihttp.Subsystem{Name: "custody", Status: apihttp.StatusUnavailable, Detail: "cofre (OpenBao) não ligado no perfil ativo"}
}

func (h healthChecker) deployment() apihttp.Subsystem {
	profile := deploy.Active()
	// A conformant profile that cannot vault keys is degraded, not ok — the honest
	// aggregate must surface it (custody above already reflects this).
	status := apihttp.StatusOK
	detail := string(profile)
	if !profile.Conformant() {
		status = apihttp.StatusDegraded
		detail = string(profile) + " (não conforme; custódia local, L3 negado — ADR-0017)"
	}
	return apihttp.Subsystem{Name: "deployment", Status: status, Detail: detail}
}
