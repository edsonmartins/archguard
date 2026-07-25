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
	"fmt"
	"net/http"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/adapters/globalaccess"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
)

// MountCapabilities registers every control-plane capability handler onto the API
// mux, each wrapped by the assurance pipeline. Called once at boot after
// InitAPIMux and InitPipeline. Every capability declares its operation
// classification (INV-8) and is mounted fail-closed where its backend is not
// available in the active profile.
func MountCapabilities() error {
	p := ActivePipeline()
	if p == nil {
		return fmt.Errorf("boot: pipeline não inicializado; chame InitPipeline antes de MountCapabilities")
	}
	f := ActiveFactory()
	if f == nil {
		return fmt.Errorf("boot: factory não inicializada; chame InitFactory antes de MountCapabilities")
	}

	if err := mountSession(p); err != nil {
		return fmt.Errorf("montar session: %w", err)
	}
	if err := mountTenants(p, f); err != nil {
		return fmt.Errorf("montar tenants: %w", err)
	}
	if err := mountMemberships(p, f); err != nil {
		return fmt.Errorf("montar memberships: %w", err)
	}
	if err := mountHealth(p, f); err != nil {
		return fmt.Errorf("montar health: %w", err)
	}
	if err := mountGrants(p, f); err != nil {
		return fmt.Errorf("montar grants: %w", err)
	}
	if err := mountAuditVerify(p, f); err != nil {
		return fmt.Errorf("montar audit-verify: %w", err)
	}
	return nil
}

// mountSession mounts GET /api/v1/session (pacote 011, T-008 — contexto da sessão
// do chamador). Classified L1: any authenticated caller may read its own context.
// The pipeline still denies an unauthenticated request (no session ⇒ deny), so the
// handler only runs for a resolved session.
func mountSession(p *Pipeline) error {
	const opID = "session.read"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "leitura do contexto da própria sessão",
	}); err != nil {
		return err
	}
	RegisterAPIHandler("/session", p.Require(opID, apihttp.NewSessionContextHandler()))
	return nil
}

// mountTenants mounts GET /api/v1/tenants (pacote 011, T-008 — seletor de tenant).
// Classified L1: the caller lists its own tenants. It reads memberships across
// tenants via the global-access path (provisional dev adapters here; the durable
// authorizer/auditor for conformant profiles — OpenFGA/trilha — is wired with the
// global-access work, T-004b/devops), so in a conformant profile the list fails
// closed until then.
func mountTenants(p *Pipeline, f *Factory) error {
	const opID = "tenants.read"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "listar os tenants do próprio chamador",
	}); err != nil {
		return err
	}
	global := postgres.NewGlobalRepository(f.Pool(), globalaccess.NewProfileAuthorizer(), globalaccess.NewMemoryAuditor())
	handler := apihttp.NewTenantsHandler(postgres.NewMembershipReader(global))
	RegisterAPIHandler("/tenants", p.Require(opID, handler))
	return nil
}

// mountMemberships mounts GET /api/v1/memberships (pacote 011, T-008 — roster do
// tenant). É uma operação de ADMINISTRAÇÃO: L1 de garantia (chamador autenticado)
// mais o gate de admin (RequireAdmin) — a autorização de console-CRUD. O escopo de
// tenant é o da sessão (o handler lê a org ativa da sessão, nunca do request).
func mountMemberships(p *Pipeline, f *Factory) error {
	const opID = "memberships.list"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "listar os membros do tenant ativo (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewMembershipsHandler(postgres.NewTenantMembershipLister(f.Pool()))
	RegisterAPIHandler("/memberships", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountGrants mounts GET /api/v1/grants (pacote 011, T-013 — concessões vigentes).
// Administration view: L1 + RequireAdmin. Lists the active privileged grants of the
// session's tenant (no PDP — listing needs only the grant store; the PDP decides
// session opening, not this read). The target asset is an opaque ref (no catalog).
func mountGrants(p *Pipeline, f *Factory) error {
	const opID = "grants.list"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "listar as concessões privilegiadas vigentes do tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewGrantsHandler(postgres.NewTenantGrantLister(f.Pool()))
	RegisterAPIHandler("/grants", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountHealth mounts GET /api/v1/health (pacote 011, T-014 — saúde dos subsistemas).
// Admin-gated operational view (L1 + RequireAdmin). Reports the honest state of the
// subsystems the composition root can observe; as PDP/audit are wired, their probes
// join the report.
func mountHealth(p *Pipeline, f *Factory) error {
	const opID = "health.read"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "saúde dos subsistemas (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewHealthHandler(healthChecker{pool: f.Pool(), factory: f})
	RegisterAPIHandler("/health", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountAuditVerify mounts GET /api/v1/audit/verify (pacote 011, T-005). The
// operation is L3 (ADR-0010): the pipeline denies it unless the caller proved L3,
// which the dev profile never does (it denies L3 by construction, ADR-0017), so
// the endpoint is inert in dev — correctly fail-closed. In conformant profiles the
// durable seal verifier (OpenBao transit) is not wired in the main repo yet, so it
// serves unavailable until that lands (devops); the dev provisional verifier is
// used only where it is appropriate.
func mountAuditVerify(p *Pipeline, f *Factory) error {
	const opID = "audit.verify"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       apihttp.AuditVerifyAssuranceLevel, // domain.L3
		Description: "verificação de integridade da trilha de auditoria",
	}); err != nil {
		return err
	}

	var handler http.Handler
	if f.Profile().IsDev() {
		sealVerifier, err := auditseal.NewProvisional()
		if err != nil {
			return err
		}
		handler = apihttp.NewAuditVerifyHandler(postgres.NewAuditVerifier(f.Pool(), sealVerifier))
	} else {
		handler = unavailableHandler("verificação de auditoria indisponível: seal verifier durável (OpenBao) não ligado")
	}

	RegisterAPIHandler("/audit/verify", p.Require(opID, handler))
	return nil
}
