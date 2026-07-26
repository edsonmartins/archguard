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
export async function cpRequest(method, path, opts = {}) {
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
 * Concessões vigentes de um tenant (para contagem regressiva / revogação).
 * @returns {Promise<{grants: Array<object>}>}
 */
export function getGrants(organizationId) {
  return cpRequest("GET", "/grants", {query: {organization_id: organizationId}});
}

// --- Auditoria (T-009) ---

/**
 * Linha do tempo de auditoria (eventos), com filtros.
 * @param {{organizationId: string, limit?: number}} params
 * @returns {Promise<{events: Array<object>}>}
 */
export function getAuditTimeline({organizationId, limit} = {}) {
  return cpRequest("GET", "/audit/timeline", {query: {organization_id: organizationId, limit}});
}

/**
 * Verificação de integridade da cadeia de auditoria (L3). Divergência vem no corpo.
 */
export function verifyAuditChain(organizationId) {
  return cpRequest("GET", "/audit/verify", {query: {organization_id: organizationId}});
}

// --- Revisão de acesso (T-012) ---

/** Acesso efetivo (origem: direto/herdado/concessão) para revisão de acesso. */
export function getEffectiveAccess(query) {
  return cpRequest("GET", "/access/effective", {query});
}

// --- Saúde dos subsistemas (T-013) ---

/** Saúde do plano de controle (PDP, cofre, auditoria) — agregado honesto. */
export function getHealth() {
  return cpRequest("GET", "/health");
}

// --- Step-up e fatores (T-005) ---

/** Conclui step-up por TOTP (eleva o nível de garantia da sessão). */
export function stepupTotp(payload) {
  return cpRequest("POST", "/stepup/totp", {body: payload});
}

/** Inicia enrollment/desafio TOTP. */
export function factorsTotpBegin(payload) {
  return cpRequest("POST", "/factors/totp/begin", {body: payload});
}

/** Verifica o código TOTP. */
export function factorsTotpVerify(payload) {
  return cpRequest("POST", "/factors/totp/verify", {body: payload});
}
