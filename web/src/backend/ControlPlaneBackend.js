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

// Camada de API do console para o PLANO DE CONTROLE (`/api/v1`, pacote 011).
// É o ÚNICO ponto por onde o console fala com o `/api/v1` — nenhuma chamada crua a
// `fetch("/api/v1/...")` deve existir fora daqui (pacote 008, T-001/T-003; I-7.6).
//
// Contrato: os handlers de `internal/http` respondem com `writeJSON(status, body)`
// (listas em `{chave: [...]}`) e erros com `writeError(status, {error})`. Sessão por
// cookie (`credentials: "include"`) — sem token em storage (ADR-0020 / segurança do
// cliente). Distinção denied×error preservada: 403 = negação (decisão), 5xx = falha.

import * as Setting from "../Setting";

const BASE = "/api/v1";

// Interceptor global de step-up (pacote 008, T-005). Uma operação que exige garantia
// maior responde 401 RFC 9470 (`WWW-Authenticate: insufficient_user_authentication`);
// o `cpRequest` chama este handler (registrado por <StepUpModal/>), que conduz o
// desafio (TOTP) e resolve `true` no sucesso — a operação ORIGINAL é então repetida,
// preservando o estado (o chamador só aguarda a promessa). `false` = cancelado.
let stepUpHandler = null;

/**
 * Registra (ou limpa com null) o handler de step-up. Recebe o contexto do desafio
 * (`needsPhishingResistant` = a operação é L3 e exige WebAuthn, não TOTP) e resolve
 * `true` no sucesso (a operação original é repetida) ou `false` se cancelado.
 * @param {null | ((challenge: {needsPhishingResistant: boolean}) => Promise<boolean>)} fn
 */
export function setStepUpHandler(fn) {
  stepUpHandler = fn;
}

// isStepUpChallenge distingue o 401 de garantia insuficiente (RFC 9470) de um 401 de
// sessão ausente/credencial — só o primeiro dispara o interceptor.
function isStepUpChallenge(res, parsed) {
  const wa = res.headers.get("WWW-Authenticate") || "";
  const err = (parsed && (parsed.error || "")) || "";
  return wa.includes("insufficient_user_authentication") || err.includes("insufficient_user_authentication");
}

// stepUpNeedsPhishingResistant lê do desafio se a operação exige fator phishing-resistant
// (L3 ⇒ WebAuthn; TOTP não satisfaz — só chega a AAL2). Sinal: `needs_phishing_resistant`
// no corpo ou `acr_values="aal3"` no header WWW-Authenticate / no corpo.
function stepUpNeedsPhishingResistant(res, parsed) {
  if (parsed && parsed.needs_phishing_resistant === true) {return true;}
  if (parsed && parsed.acr_values === "aal3") {return true;}
  const wa = res.headers.get("WWW-Authenticate") || "";
  return /acr_values="?aal3/i.test(wa);
}

/**
 * Erro de chamada ao plano de controle. `status` distingue negação (403) de falha (5xx).
 */
export class ControlPlaneError extends Error {
  constructor(message, status, body) {
    super(message);
    this.name = "ControlPlaneError";
    this.status = status;
    this.body = body;
    /** Decisão de negação (não é falha de infraestrutura). */
    this.denied = status === 401 || status === 403;
  }
}

/**
 * Requisição base ao `/api/v1`. Devolve o JSON já parseado; lança `ControlPlaneError`
 * em status não-2xx (fail-closed — o chamador nunca trata erro como sucesso).
 * @param {"GET"|"POST"|"PUT"|"DELETE"} method
 * @param {string} path caminho relativo ao `/api/v1` (ex.: "/tenants")
 * @param {{query?: Record<string, string|number|undefined>, body?: unknown}} [opts]
 * @returns {Promise<any>}
 */
export async function cpRequest(method, path, opts = {}, _retried = false) {
  const {query, body} = opts;
  let url = `${Setting.ServerUrl}${BASE}${path}`;
  if (query) {
    const qs = Object.entries(query)
      .filter(([, v]) => v !== undefined && v !== null && v !== "")
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
      .join("&");
    if (qs) {url += `?${qs}`;}
  }

  const init = {
    method,
    credentials: "include",
    headers: {
      "Accept": "application/json",
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  };
  if (body !== undefined) {
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }

  const res = await fetch(url, init);
  const text = await res.text();
  let parsed = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = {raw: text};
    }
  }
  if (!res.ok) {
    // Step-up transparente (T-005): 401 de garantia insuficiente ⇒ conduz o desafio e
    // repete a operação UMA vez. O handler recebe se a operação exige fator phishing-
    // resistant (WebAuthn, L3) vs TOTP (L2). Nunca nos próprios /stepup/* (evita recursão).
    const isStepUpPath = path === "/stepup/totp" || path === "/stepup/webauthn/begin" || path === "/stepup/webauthn/finish";
    if (res.status === 401 && !_retried && !isStepUpPath && stepUpHandler && isStepUpChallenge(res, parsed)) {
      const ok = await stepUpHandler({needsPhishingResistant: stepUpNeedsPhishingResistant(res, parsed)});
      if (ok) {
        return cpRequest(method, path, opts, true);
      }
    }
    const msg = (parsed && (parsed.error || parsed.msg)) || `HTTP ${res.status}`;
    throw new ControlPlaneError(msg, res.status, parsed);
  }
  return parsed;
}

// --- Contexto de sessão e tenants (T-004) ---

/** Contexto de sessão do plano de controle (identidade, memberships, nível de garantia). */
export function getSessionContext() {
  return cpRequest("GET", "/session");
}

/**
 * Tenants (organizações) do usuário atual, cada um com `active` (o tenant da sessão)
 * e `display_name` (nome amigável; cai no organization_id se ausente).
 * @returns {Promise<{tenants: Array<{membership_id: string, organization_id: string, display_name: string, status: string, active: boolean}>}>}
 */
export function getTenants() {
  return cpRequest("GET", "/tenants");
}

/**
 * Troca o tenant ATIVO da sessão para `organizationId` (reemite o token; a sessão
 * por cookie passa a apontar para o novo tenant — o console recarrega `/session`).
 * Um destino mais restritivo lança `ControlPlaneError` com `.status === 401` (step-up
 * necessário — RFC 9470); um destino do qual o usuário não é membro ativo lança 403.
 * @param {string} organizationId
 * @returns {Promise<object>} novo contexto de sessão
 */
export function switchTenant(organizationId) {
  return cpRequest("POST", "/session/tenant", {body: {organization_id: organizationId}});
}

// --- Memberships (telas herdadas + revogação) ---

/** Memberships de um tenant. */
export function getMemberships(organizationId) {
  return cpRequest("GET", "/memberships", {query: {organization_id: organizationId}});
}

/** Revoga uma membership (operação privilegiada; pode exigir step-up). */
export function revokeMembership(payload) {
  return cpRequest("POST", "/memberships/revoke", {body: payload});
}

// --- Concessões privilegiadas (T-006) ---

/**
 * Concessões privilegiadas vigentes do TENANT ATIVO da sessão (o backend lê a org da
 * sessão, nunca do request). Cada item traz `grant_id`, `target_type`/`target_id`/
 * `target_scope`, `origin`, `status`, `not_before` e `expires_at` (Unix) — base para a
 * contagem regressiva e a revogação.
 * @returns {Promise<{grants: Array<{grant_id: string, target_type: string, target_id: string, target_scope?: string, origin: string, status: string, not_before: number, expires_at: number}>}>}
 */
export function getGrants() {
  return cpRequest("GET", "/grants");
}

/**
 * Revoga uma concessão privilegiada VIGENTE do tenant ATIVO (POST; operação L3 → o
 * interceptor de step-up da T-005 conduz o desafio transparente). É DESTRUTIVA: revoga
 * a concessão E encerra as sessões derivadas dela. O backend lê a org da sessão; só o
 * `grantId` vai no corpo. 404 = inexistente/outro tenant; 409 = já não ativa; 403 = negada.
 * @param {string} grantId
 * @returns {Promise<{revoked: boolean}>}
 */
export function revokeGrant(grantId) {
  return cpRequest("POST", "/grants/revoke", {body: {grant_id: grantId}});
}

// --- Break-glass (T-007/T-008) ---

/**
 * Abre uma solicitação de acesso de emergência (break-glass) para o PRÓPRIO operador
 * (POST; operação L3 → o interceptor de step-up da T-005 conduz o desafio transparente).
 * Fail-closed: se o tenant não tem canal de notificação → 503 (a solicitação é negada, o
 * alerta é pré-condição). O sujeito é o membership da sessão; a org vem da sessão. Campos:
 * alvo opaco (`target_type`/`target_id`/`target_scope?`), `justification` e `incident_ref`
 * obrigatórios, `expires_at` (Unix — a janela do acesso).
 * @param {{target_type: string, target_id: string, target_scope?: string, justification: string, incident_ref: string, expires_at: number}} payload
 * @returns {Promise<{requested: boolean}>}
 */
export function requestBreakglass(payload) {
  return cpRequest("POST", "/breakglass/request", {body: payload});
}

/**
 * Solicitações de break-glass do TENANT ATIVO aguardando aprovação (fila da T-008). Cada
 * item traz o membership solicitante, o alvo, a **justificativa e o incidente** (o aprovador
 * decide com eles) e `required_approvals`. Leitura L1 (admin); a org vem da sessão.
 * @returns {Promise<{pending: Array<{grant_id: string, subject_membership_id: string, target_type: string, target_id: string, target_scope?: string, justification: string, incident_ref: string, required_approvals: number, not_before: number, expires_at: number}>}>}
 */
export function getBreakglassPending() {
  return cpRequest("GET", "/breakglass/pending");
}

/**
 * Registra a aprovação do CHAMADOR numa solicitação de break-glass (POST; operação L3 →
 * step-up transparente da T-005/T-005b). Separação de deveres imposta pelo backend: o
 * solicitante não pode aprovar (403), aprovador repetido (409); ao atingir o quórum a
 * concessão é ativada. 404 = inexistente/outro tenant; 409 = não está aguardando.
 * @param {string} grantId
 * @returns {Promise<{approved: boolean}>}
 */
export function approveBreakglass(grantId) {
  return cpRequest("POST", "/breakglass/approve", {body: {grant_id: grantId}});
}

// --- Auditoria (T-009) ---

/**
 * Linha do tempo de auditoria do TENANT ATIVO (o backend lê a org da sessão). `limit` é
 * limitado a [1, 200] (default 50). Eventos: seq, occurred_at (Unix), action, outcome,
 * actor_subject (opaco), target_*, reason, pcid — sem hashes da cadeia, sem dado pessoal.
 * @param {{limit?: number}} [params]
 * @returns {Promise<{events: Array<{seq: number, occurred_at: number, action: string, outcome: string, actor_subject: string, target_type?: string, target_id?: string, target_label?: string, reason?: string, pcid?: string}>}>}
 */
export function getAuditTimeline({limit} = {}) {
  return cpRequest("GET", "/audit/timeline", {query: {limit}});
}

/**
 * Verificação de integridade da cadeia de auditoria do TENANT ATIVO (op **L3** → step-up
 * WebAuthn transparente; a org vem da sessão). Íntegra → resolve `{ok:true, events_checked,
 * seals_checked, ...}`; **divergência → 409**, e o `ControlPlaneError.body` traz
 * `{ok:false, first_divergence_seq, divergence_kind, detail}`.
 * @returns {Promise<{ok: boolean, events_checked: number, seals_checked: number, seal_signatures_checked: boolean}>}
 */
export function verifyAuditChain() {
  return cpRequest("GET", "/audit/verify");
}

// --- Revisão de acesso (T-012) ---

/** Acesso efetivo (origem: direto/herdado/concessão) para revisão de acesso. */
export function getEffectiveAccess(query) {
  return cpRequest("GET", "/access/effective", {query});
}

/** Revisão de acesso a um ativo: quem o alcança e por qual origem (direto/herdado/concessão). */
export function getAccessReview(assetId) {
  return cpRequest("GET", "/access/review", {query: {asset: assetId}});
}

/** Lista os ativos do tenant ativo (para escolher o alvo da revisão). */
export function getAssets() {
  return cpRequest("GET", "/assets");
}

// --- Gestão de acesso: ativos, atribuições e vínculos de grupo (M4 D1/D2) ---

/** Registra um ativo no tenant ativo (kind + name obrigatórios). */
export function createAsset(payload) {
  return cpRequest("POST", "/assets", {body: payload});
}

/** Atribuições granulares (subject operator/auditor sobre asset/asset_group). */
export function getAccessAssignments() {
  return cpRequest("GET", "/access-assignments");
}

/** Cria uma atribuição: {subject_type, subject_id, relation, object_type, object_id}. */
export function createAccessAssignment(payload) {
  return cpRequest("POST", "/access-assignments", {body: payload});
}

/** Vínculos membership↔grupo de acesso do tenant ativo. */
export function getGroupMemberships() {
  return cpRequest("GET", "/group-memberships");
}

/** Vincula um membership a um grupo: {group_id, membership_id}. */
export function createGroupMembership(payload) {
  return cpRequest("POST", "/group-memberships", {body: payload});
}

/** Catálogo de grupos de acesso do tenant (nome↔id). */
export function getAccessGroups() {
  return cpRequest("GET", "/access-groups");
}

/** Nomeia um grupo de acesso: {name, display_name?}. */
export function createAccessGroup(payload) {
  return cpRequest("POST", "/access-groups", {body: payload});
}

// --- Saúde dos subsistemas (T-013) ---

/** Saúde do plano de controle (PDP, cofre, auditoria) — agregado honesto. */
export function getHealth() {
  return cpRequest("GET", "/health");
}

// --- Step-up e fatores (T-005) ---

/** Conclui step-up por TOTP (eleva a sessão a AAL2). Não satisfaz operações L3. */
export function stepupTotp(payload) {
  return cpRequest("POST", "/stepup/totp", {body: payload});
}

/**
 * Inicia o step-up por WebAuthn (fator phishing-resistant; único que satisfaz L3).
 * Devolve as opções de asserção (`{publicKey: {...}}`) para o `navigator.credentials.get`.
 * @returns {Promise<{publicKey: object}>}
 */
export function stepupWebauthnBegin() {
  return cpRequest("POST", "/stepup/webauthn/begin");
}

/**
 * Conclui o step-up por WebAuthn enviando a asserção do autenticador. No sucesso, eleva
 * a sessão ao AAL do autenticador (AAL3 hardware / AAL2 passkey) — phishing-resistant.
 * @param {object} assertion asserção no formato WebAuthn (id/rawId/type/response)
 * @returns {Promise<{aal: string}>}
 */
export function stepupWebauthnFinish(assertion) {
  return cpRequest("POST", "/stepup/webauthn/finish", {body: assertion});
}

/** Inicia enrollment/desafio TOTP. */
export function factorsTotpBegin(payload) {
  return cpRequest("POST", "/factors/totp/begin", {body: payload});
}

/** Verifica o código TOTP. */
export function factorsTotpVerify(payload) {
  return cpRequest("POST", "/factors/totp/verify", {body: payload});
}
