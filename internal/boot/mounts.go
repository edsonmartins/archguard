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

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/adapters/auditseal"
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
	if err := mountServiceContext(f); err != nil {
		return fmt.Errorf("montar service-context: %w", err)
	}
	if err := mountTenants(p, f); err != nil {
		return fmt.Errorf("montar tenants: %w", err)
	}
	if err := mountSessionSwitch(p, f); err != nil {
		return fmt.Errorf("montar session-switch: %w", err)
	}
	if err := mountMemberships(p, f); err != nil {
		return fmt.Errorf("montar memberships: %w", err)
	}
	if err := mountAssets(p, f); err != nil {
		return fmt.Errorf("montar assets: %w", err)
	}
	if err := mountAssetAccess(p, f); err != nil {
		return fmt.Errorf("montar access-assignments: %w", err)
	}
	if err := mountGroupMemberships(p, f); err != nil {
		return fmt.Errorf("montar group-memberships: %w", err)
	}
	if err := mountAccessGroups(p, f); err != nil {
		return fmt.Errorf("montar access-groups: %w", err)
	}
	if err := mountHealth(p, f); err != nil {
		return fmt.Errorf("montar health: %w", err)
	}
	if err := mountGrants(p, f); err != nil {
		return fmt.Errorf("montar grants: %w", err)
	}
	if err := mountGrantRevoke(p, f); err != nil {
		return fmt.Errorf("montar grant-revoke: %w", err)
	}
	if err := mountBreakglassRequest(p, f); err != nil {
		return fmt.Errorf("montar breakglass-request: %w", err)
	}
	if err := mountBreakglassPending(p, f); err != nil {
		return fmt.Errorf("montar breakglass-pending: %w", err)
	}
	if err := mountBreakglassApprove(p, f); err != nil {
		return fmt.Errorf("montar breakglass-approve: %w", err)
	}
	if err := mountAccessReview(p, f); err != nil {
		return fmt.Errorf("montar access-review: %w", err)
	}
	if err := mountFactorEnrollment(p, f); err != nil {
		return fmt.Errorf("montar factor-enrollment: %w", err)
	}
	if err := mountStepUp(p, f); err != nil {
		return fmt.Errorf("montar step-up: %w", err)
	}
	if err := mountStepUpWebAuthn(p, f); err != nil {
		return fmt.Errorf("montar step-up-webauthn: %w", err)
	}
	if err := mountMembershipRevoke(p, f); err != nil {
		return fmt.Errorf("montar membership-revoke: %w", err)
	}
	if err := mountAuditVerify(p, f); err != nil {
		return fmt.Errorf("montar audit-verify: %w", err)
	}
	if err := mountAuditTimeline(p, f); err != nil {
		return fmt.Errorf("montar audit-timeline: %w", err)
	}
	return nil
}

// mountServiceContext mounts the narrow machine-to-machine identity lookup used
// by ArchGate. It is deliberately outside the browser assurance pipeline: the
// caller authenticates with the dedicated ARCHGUARD_SERVICE_TOKEN and supplies
// only an opaque OIDC subject. No e-mail or tenant chosen by the caller crosses
// this boundary.
func mountServiceContext(f *Factory) error {
	if f == nil || f.Pool() == nil {
		RegisterAPIHandler("/service/session-context", unavailableHandler("service context indisponível: banco não ligado"))
		return nil
	}
	global := newGlobalRepository(f.Pool())
	handler := apihttp.NewServiceContextHandler(
		conf.GetConfigString("ARCHGUARD_SERVICE_TOKEN"),
		postgres.NewIdentityStore(f.Pool()),
		postgres.NewMembershipReader(global),
	)
	RegisterAPIHandler("/service/session-context", handler)
	return nil
}

// mountAuditTimeline mounts GET /api/v1/audit/timeline (pacote 011, T-012 — visão de
// auditoria). Admin read (L1 + RequireAdmin): the caller's tenant's recent audit
// events, newest first. Read-only over the append-only trail. Now that the trail is
// written (membership.revoke), it has events to show.
func mountAuditTimeline(p *Pipeline, f *Factory) error {
	const opID = "audit.timeline.read"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "ler a linha do tempo de auditoria do tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewAuditTimelineHandler(postgres.NewTenantAuditReader(f.Pool()))
	RegisterAPIHandler("/audit/timeline", p.Require(opID, apihttp.RequireAdmin(handler)))
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
// Classified L1: the caller lists its own tenants. It reads memberships across tenants
// via the global-access path with the REAL controls (newGlobalRepository, ADR-0022):
// the read is ScopeSelf, so it is authorized in any profile (the caller lists ITS OWN
// tenants) and durably audited.
func mountTenants(p *Pipeline, f *Factory) error {
	const opID = "tenants.read"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "listar os tenants do próprio chamador",
	}); err != nil {
		return err
	}
	global := newGlobalRepository(f.Pool())
	handler := apihttp.NewTenantsHandler(postgres.NewMembershipReader(global), postgres.NewOrgDisplayNamer(f.Pool()))
	RegisterAPIHandler("/tenants", p.Require(opID, handler))
	return nil
}

// mountSessionSwitch mounts POST /api/v1/session/tenant (pacote 008, T-004 — troca
// de tenant). Classified L1: qualquer sessão autenticada pode trocar entre os SEUS
// tenants; o step-up quando o destino é mais restritivo é decidido pela POLÍTICA DO
// DESTINO dentro do switcher (ErrStepUpRequired → 401 RFC 9470 no handler), não pelo
// nível de operação do middleware. A troca reemite a geração de token (invalida a
// anterior) e audita a troca atomicamente (design 002).
func mountSessionSwitch(p *Pipeline, f *Factory) error {
	const opID = "session.switch_tenant"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "trocar o tenant ativo da própria sessão",
	}); err != nil {
		return err
	}
	RegisterAPIHandler("/session/tenant", p.Require(opID, apihttp.NewSessionSwitchHandler(newTenantSwitch(f))))
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

// mountAssets mounts GET/POST /api/v1/assets (pacote 007 M4, T-026 — catálogo de
// ativos do tenant). Operação de ADMINISTRAÇÃO: L1 + RequireAdmin (console-CRUD). O
// escopo é o tenant ativo da sessão (nunca do request, INV-1). A criação enfileira a
// projeção de autorização na mesma transação (o publisher a drena depois).
func mountAssets(p *Pipeline, f *Factory) error {
	const opID = "assets.catalog"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "catálogo de ativos do tenant ativo (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewAssetsHandler(postgres.NewAssetCatalog(f.Pool()))
	RegisterAPIHandler("/assets", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountAssetAccess mounts GET/POST /api/v1/access-assignments (pacote 007 M4, T-029 —
// atribuição granular de acesso: membership → operator/auditor → asset/asset_group).
// Operação de ADMINISTRAÇÃO: L1 + RequireAdmin. A criação enfileira a projeção na mesma
// transação (o publisher a drena; o PDP passa a derivar operator/herdado).
func mountAssetAccess(p *Pipeline, f *Factory) error {
	const opID = "access.assignments"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "atribuições granulares de acesso do tenant ativo (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewAssetAccessHandler(postgres.NewAssetAccessCatalog(f.Pool()))
	RegisterAPIHandler("/access-assignments", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountGroupMemberships mounts GET/POST /api/v1/group-memberships (pacote 007 M4, T-029 D1
// — vínculo membership↔grupo de acesso). Operação de ADMINISTRAÇÃO: L1 + RequireAdmin. A
// criação enfileira a projeção `member` na mesma tx (o membro herda o acesso do grupo).
func mountGroupMemberships(p *Pipeline, f *Factory) error {
	const opID = "group.memberships"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "vínculos membership↔grupo de acesso do tenant ativo (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewGroupMembershipHandler(postgres.NewGroupMembershipCatalog(f.Pool()))
	RegisterAPIHandler("/group-memberships", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountAccessGroups mounts GET/POST /api/v1/access-groups (pacote 007 M4, D1 catálogo — o
// catálogo de grupos de acesso: nome↔id). Operação de ADMINISTRAÇÃO: L1 + RequireAdmin.
func mountAccessGroups(p *Pipeline, f *Factory) error {
	const opID = "access.groups"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "catálogo de grupos de acesso do tenant ativo (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewAccessGroupHandler(postgres.NewAccessGroupCatalog(f.Pool()))
	RegisterAPIHandler("/access-groups", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountMembershipRevoke mounts POST /api/v1/memberships/revoke (pacote 011, T-008 —
// primeira escrita L2). Classified L2 (membership.revoke, audit_event.go): it
// requires an AAL2 session, so a password-only admin must step up first — the full
// chain (challenge → step-up → retry). Plus the admin gate. The mutation revokes
// the membership, ends the member's tenant sessions and writes a membership.revoke
// event to the immutable trail, atomically (I-5.4).
func mountMembershipRevoke(p *Pipeline, f *Factory) error {
	const opID = "membership.revoke"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L2,
		Description: "revogar um membership do tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewMembershipRevokeHandler(newMembershipRevoker(f))
	RegisterAPIHandler("/memberships/revoke", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountStepUp mounts POST /api/v1/stepup/totp (pacote 011, T-007/005 — cerimônia de
// step-up TOTP → L2). auth.stepup is EXEMPT from operation classification
// (operation_catalog): it is how the level is raised, so it is gated by its own
// factor proof, not by the middleware. It uses RequireSession (resolve the session,
// no level gate), not Require. Fail-closed where the vault is unavailable.
func mountStepUp(p *Pipeline, f *Factory) error {
	stepUp, err := newTOTPStepUp(f)
	if err != nil {
		RegisterAPIHandler("/stepup/totp", p.RequireSession(unavailableHandler("step-up indisponível: cofre de segredos não ligado")))
		return nil
	}
	handler := apihttp.NewStepUpHandler(stepUp)
	RegisterAPIHandler("/stepup/totp", p.RequireSession(http.HandlerFunc(handler.TOTP)))
	return nil
}

// mountStepUpWebAuthn mounts POST /api/v1/stepup/webauthn/{begin,finish} (pacote 008,
// T-005b — step-up phishing-resistant). Like the TOTP step-up it is exempt from the
// assurance middleware (the assertion IS the control) and uses RequireSession, not Require.
// This is the ONLY step-up that satisfies an L3 operation (grant.revoke, breakglass.*):
// TOTP caps at AAL2. Fail-closed where the relying party cannot be configured.
func mountStepUpWebAuthn(p *Pipeline, f *Factory) error {
	stepUp, err := newWebauthnStepUp(f)
	if err != nil {
		RegisterAPIHandler("/stepup/webauthn/begin", p.RequireSession(unavailableHandler("step-up WebAuthn indisponível: relying party não configurado")))
		RegisterAPIHandler("/stepup/webauthn/finish", p.RequireSession(unavailableHandler("step-up WebAuthn indisponível: relying party não configurado")))
		return nil
	}
	handler := apihttp.NewStepUpWebAuthnHandler(stepUp)
	RegisterAPIHandler("/stepup/webauthn/begin", p.RequireSession(http.HandlerFunc(handler.Begin)))
	RegisterAPIHandler("/stepup/webauthn/finish", p.RequireSession(http.HandlerFunc(handler.Finish)))
	return nil
}

// mountFactorEnrollment mounts POST /api/v1/factors/totp/{begin,verify} (pacote
// 011, T-008/005 — inscrição de fator). Classified L1 and AllowedDuringEnrollment:
// a caller with only a password must be able to enroll a strong factor (the
// prerequisite for step-up), even under the mandatory-enrollment block. Fail-closed
// where the vault is unavailable (conformant without OpenBao).
func mountFactorEnrollment(p *Pipeline, f *Factory) error {
	const opID = "factors.totp.enroll"
	if err := p.RegisterOperation(domain.Operation{
		ID:                      opID,
		Level:                   domain.L1,
		Description:             "inscrição de fator TOTP",
		AllowedDuringEnrollment: true,
	}); err != nil {
		return err
	}

	enroller, err := newTOTPEnroller(f)
	if err != nil {
		unavailable := unavailableHandler("inscrição de fator indisponível: cofre de segredos não ligado no perfil ativo")
		RegisterAPIHandler("/factors/totp/begin", p.Require(opID, unavailable))
		RegisterAPIHandler("/factors/totp/verify", p.Require(opID, unavailable))
		return nil
	}
	handler := apihttp.NewFactorsHandler(enroller)
	RegisterAPIHandler("/factors/totp/begin", p.Require(opID, http.HandlerFunc(handler.BeginTOTP)))
	RegisterAPIHandler("/factors/totp/verify", p.Require(opID, http.HandlerFunc(handler.FinishTOTP)))
	return nil
}

// mountAccessReview mounts the access-review endpoints over the PostgresPDP (pacote
// 007): GET /api/v1/access/effective (a single membership×asset decision) and GET
// /api/v1/access/review (the reverse query: who reaches an asset and by which origin —
// direct/inherited/grant — the certification-campaign view, 008 T-012). Admin-gated
// (L1 + admin); the tenant is the session's active org.
//
// The PDP reads the authz_tuple projection, now POPULATED at runtime (M4 Fase A+B+C:
// asset catalog + grant→tuple + publisher). It decides on real data; it remains
// fail-closed (denies / 503) when the PDP cannot answer.
func mountAccessReview(p *Pipeline, f *Factory) error {
	const opID = "access.review"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "revisão de acesso efetivo (PDP) — administração",
	}); err != nil {
		return err
	}
	pdp := postgres.NewPostgresPDP(f.Pool())
	RegisterAPIHandler("/access/effective", p.Require(opID, apihttp.RequireAdmin(apihttp.NewAccessHandler(pdp))))
	RegisterAPIHandler("/access/review", p.Require(opID, apihttp.RequireAdmin(apihttp.NewAccessReviewHandler(pdp))))
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

// mountGrantRevoke mounts POST /api/v1/grants/revoke (pacote 008, T-006 Parte B —
// revogação de concessão privilegiada). Administration write: L3 + RequireAdmin. Revokes
// an active grant of the session's tenant and cascade-revokes its derived sessions,
// atomically with the audit event (I-5.4). L3 requires step-up (RFC 9470), which the
// console conducts transparently (T-005). The grant is scoped to the caller's active
// tenant by the RLS (INV-5), never taken from the request.
func mountGrantRevoke(p *Pipeline, f *Factory) error {
	const opID = "grant.revoke"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L3,
		Description: "revogar uma concessão privilegiada do tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewGrantRevokeHandler(newGrantRevoker(f))
	RegisterAPIHandler("/grants/revoke", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountBreakglassRequest mounts POST /api/v1/breakglass/request (pacote 008, T-007 —
// solicitação de break-glass). Administration write: L3 + RequireAdmin. Opens an
// emergency-access grant for the caller over an opaque target with a mandatory
// justification and incident reference, fail-closed on the notification channel (the
// alert fires at request time) and on the audit (atomic with the grant). L3 requires
// step-up (RFC 9470), conducted transparently by the console (T-005), and is denied
// outright in the dev profile. The subject and organization come from the session (INV-1).
func mountBreakglassRequest(p *Pipeline, f *Factory) error {
	const opID = "breakglass.request"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L3,
		Description: "solicitar acesso de emergência (break-glass) no tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewBreakglassRequestHandler(newBreakglassRequester(f))
	RegisterAPIHandler("/breakglass/request", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountBreakglassPending mounts GET /api/v1/breakglass/pending (pacote 008, T-008 — fila de
// aprovação). Administration read: L1 + RequireAdmin. Lists the session tenant's break-glass
// grants awaiting peer approval, with justification and incident so the approver can decide.
func mountBreakglassPending(p *Pipeline, f *Factory) error {
	const opID = "breakglass.pending"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L1,
		Description: "listar as solicitações de break-glass aguardando aprovação (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewBreakglassPendingHandler(postgres.NewTenantGrantLister(f.Pool()))
	RegisterAPIHandler("/breakglass/pending", p.Require(opID, apihttp.RequireAdmin(handler)))
	return nil
}

// mountBreakglassApprove mounts POST /api/v1/breakglass/approve (pacote 008, T-008 —
// aprovação com separação de deveres). Administration write: L3 + RequireAdmin. Records the
// caller's approval; the domain refuses self-approval and duplicate approvers and activates
// the grant on quorum, atomically with the audit. L3 requires step-up (conduzido transparente
// pela T-005/T-005b) e é negado em dev. Aprovador e tenant vêm da sessão (INV-1).
func mountBreakglassApprove(p *Pipeline, f *Factory) error {
	const opID = "breakglass.approve"
	if err := p.RegisterOperation(domain.Operation{
		ID:          opID,
		Level:       domain.L3,
		Description: "aprovar uma solicitação de break-glass do tenant (administração)",
	}); err != nil {
		return err
	}
	handler := apihttp.NewBreakglassApproveHandler(newBreakglassApprover(f))
	RegisterAPIHandler("/breakglass/approve", p.Require(opID, apihttp.RequireAdmin(handler)))
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
