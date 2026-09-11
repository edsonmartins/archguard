// W-C2 — provision + grant person access (orchestration + Warpgate live + evidence)
//
// Warpgate/sites are loaded via dynamic import inside handlers so the client
// Vite graph never pulls node:https / better-sqlite3 (see docker build).

import { createServerFn, createServerOnlyFn } from '@tanstack/react-start'
import { z } from 'zod'
import { recordActivity } from './activity-log'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { assertPrincipalTenantAccess, assertSiteTenantAccess, hasAnyPerm } from './session-guard'
import { deriveTenants, stripGroupDomain } from '@/lib/auth/roles'
import { logger } from './logger'
import { integrationFetch } from './http-integration-client'
import { addUserToGroup } from './idp'
import { openFgaConnectionObject, openFgaEnabled, writeOpenFgaGrant } from './openfga'
import { createAccessGrant, getAccessGrant, listAccessGrantsForPrincipal, revokeAccessGrant } from './db'
import { randomUUID } from 'node:crypto'

const ORCH_URL = (
  process.env.ORCHESTRATION_URL ||
  process.env.ARCHGATE_ORCHESTRATION_URL ||
  'http://archgate-orchestration:8090'
).replace(/\/$/, '')

export type LifecycleStep = {
  component: string
  ok: boolean
  detail?: string
}

async function orchPost(
  path: string,
  body: unknown,
): Promise<{ status: number; data: Record<string, unknown>; text: string }> {
  const res = await integrationFetch(`${ORCH_URL}${path}`, {
    method: 'POST',
    integration: 'orchestration',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const text = await res.text()
  let data: Record<string, unknown> = {}
  try {
    data = text ? (JSON.parse(text) as Record<string, unknown>) : {}
  } catch {
    data = { raw: text }
  }
  return { status: res.status, data, text }
}

/** Bind a person to a group on the active IdP (archguard or ArchGuard). */
async function bindGroup(
  username: string,
  group: string,
): Promise<LifecycleStep> {
  const step = await addUserToGroup(username, group)
  return { component: 'idp_group', ok: step.ok, detail: step.detail }
}

/**
 * Ensure person exists in platform adapters (orch) + tenant/groups in archguard.
 * Person must already exist in archguard (created via console identities).
 */
export const provisionPersonAccessFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(128),
        email: z.string().email().optional().or(z.literal('')),
        tenant_slug: z.string().min(1).max(128),
        profile: z.string().max(64).optional(),
        groups: z.array(z.string()).default([]),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['persons:update', 'persons:create', 'system:admin'],
      'persons:update',
    )
    const actor = sessionActor(s)
    const steps: LifecycleStep[] = []
    const tenant = data.tenant_slug.startsWith('tenant_')
      ? data.tenant_slug
      : `tenant_${data.tenant_slug}`
    if (!hasAnyPerm(s, ['system:admin'])) {
      const operatorTenants = new Set(deriveTenants(s.groups).map(stripGroupDomain))
      if (!operatorTenants.has(tenant)) throw new Error('Forbidden: tenant fora do escopo do operador')
      await assertPrincipalTenantAccess(data.username, s)
      const unsafe = data.groups.filter((group) => {
        const normalized = stripGroupDomain(group).toLowerCase()
        return !['archguard_users', 'archguard_viewers', tenant].includes(normalized)
      })
      if (unsafe.length) throw new Error('Forbidden: grupos administrativos só podem ser atribuídos por system:admin')
    }
    const groups = Array.from(
      new Set([
        ...data.groups,
        tenant,
        'archguard_users',
      ].filter(Boolean)),
    )

    // Orch multi-adapter
    try {
      const { status, data: body, text } = await orchPost(
        '/orchestration/v1/users/provision',
        {
          tenant_slug: data.tenant_slug.replace(/^tenant_/, ''),
          username: data.username,
          email: data.email || '',
          profile: data.profile || 'operator',
          groups,
        },
      )
      const st = String(body.status || '')
      steps.push({
        component: 'orchestration',
        ok:
          status >= 200 &&
          status < 300 &&
          (st === 'ok' || st === 'partial' || !st),
        detail:
          status >= 200 && status < 300
            ? [st || 'ok', ...(Array.isArray(body.steps) ? body.steps : [])]
                .join(' · ')
                .slice(0, 400)
            : `HTTP ${status}: ${text.slice(0, 160)}`,
      })
    } catch (e) {
      steps.push({
        component: 'orchestration',
        ok: false,
        detail: (e as Error).message,
      })
    }

    // Direct archguard membership (real path even if orch mock)
    for (const g of groups) {
      steps.push(await bindGroup(data.username, g))
    }

    const critical = steps.some(
      (x) => x.component === 'idp_group' && x.ok,
    )
    recordActivity(
      'POST',
      `/archgate/persons/${encodeURIComponent(data.username)}/provision`,
      actor,
      critical ? 'success' : 'error',
      undefined,
      {
        tenant,
        steps: steps.map((x) => `${x.component}:${x.ok ? 'ok' : 'fail'}`).join(','),
      },
    )
    logger.info({ username: data.username, actor, critical }, 'provision access')

    return {
      ok: critical,
      username: data.username,
      steps,
      message: critical
        ? `Acesso provisionado para ${data.username} (grupos/tenant)`
        : `Provision parcial/falhou para ${data.username}`,
    }
  })

/**
 * Resolve Warpgate role names that should unlock a target for an operator.
 * Priority: explicit role → WG target roles → site SoT target/site roles.
 */
export const resolveGrantRoles = createServerOnlyFn(async function resolveGrantRoles(
  target: string,
  explicitRole?: string,
): Promise<{ roles: string[]; detail: string }> {
  if (explicitRole?.trim()) {
    return {
      roles: [explicitRole.trim()],
      detail: `explicit role ${explicitRole.trim()}`,
    }
  }

  const { rolesForWarpgateTarget } = await import('./warpgate-proxy')
  const fromWg = await rolesForWarpgateTarget(target)
  if (fromWg.roles.length > 0) {
    return { roles: fromWg.roles, detail: fromWg.detail }
  }

  // SoT: site inventory (works even if WG target.allow_roles empty)
  try {
    const { listSites } = await import('./sites')
    const sites = await listSites()
    for (const site of sites) {
      const hit = site.targets?.find((x) => x.nome === target)
      if (!hit) continue
      const fromTarget = hit.roles?.filter(Boolean) || []
      const fromSite = site.warpgate_roles?.filter(Boolean) || []
      const roles = Array.from(new Set([...fromTarget, ...fromSite]))
      if (roles.length) {
        return {
          roles,
          detail: `SoT site ${site.slug}: ${roles.join(',')}`,
        }
      }
      return {
        roles: [],
        detail: `target in site ${site.slug} but no warpgate_roles`,
      }
    }
  } catch (e) {
    return {
      roles: fromWg.roles,
      detail: `SoT error: ${(e as Error).message}; ${fromWg.detail}`,
    }
  }

  return {
    roles: [],
    detail: fromWg.detail || `no roles for target ${target}`,
  }
})

export type GrantPersonTargetInput = {
  username: string
  /** Stable identity key when the caller has resolved the person record. */
  identity_id?: string
  target: string
  /** Optional Warpgate role name (skip auto-resolve). */
  role?: string
  ttl?: string
}

export type GrantPersonTargetResult = {
  ok: boolean
  username: string
  target: string
  steps: LifecycleStep[]
  message: string
}

const GRANT_TTL_MIN_SECONDS = 60
const GRANT_TTL_MAX_SECONDS = 24 * 60 * 60

export function grantTtlSeconds(value?: string): number {
  const raw = (value || '8h').trim().toLowerCase()
  const match = /^(\d+)\s*(s|m|h|d)$/.exec(raw)
  if (!match) throw new Error('TTL inválido; use, por exemplo, 30m, 8h ou 1d')
  const amount = Number(match[1])
  const factor = match[2] === 's' ? 1 : match[2] === 'm' ? 60 : match[2] === 'h' ? 3600 : 86400
  const seconds = amount * factor
  if (!Number.isSafeInteger(seconds) || seconds < GRANT_TTL_MIN_SECONDS || seconds > GRANT_TTL_MAX_SECONDS) {
    throw new Error('TTL fora do intervalo permitido (1 minuto a 24 horas)')
  }
  return seconds
}

/**
 * Core grant logic (Warpgate live bind + orch best-effort).
 * Used by Manager UI server-fn and lab smoke API.
 */
export const runGrantPersonTarget = createServerOnlyFn(async function runGrantPersonTarget(
  data: GrantPersonTargetInput,
  actor: string,
): Promise<GrantPersonTargetResult> {
  const steps: LifecycleStep[] = []
  const ttlSeconds = grantTtlSeconds(data.ttl)
  const expiresAt = new Date(Date.now() + ttlSeconds * 1000).toISOString()
  let grantSubject: string | undefined
  let grantObject: string | undefined

  // 1) Live Warpgate path (real grant even when orch is mock)
  const { warpgateConfigured, bindWarpgateUserRole } = await import(
    './warpgate-proxy'
  )
  if (warpgateConfigured()) {
    const resolved = await resolveGrantRoles(data.target, data.role)
    steps.push({
      component: 'role_resolve',
      ok: resolved.roles.length > 0,
      detail: resolved.detail,
    })
    if (resolved.roles.length === 0) {
      steps.push({
        component: 'warpgate',
        ok: false,
        detail:
          'Nenhuma role WG para o target — aplique gateways no site ou informe role=',
      })
    } else {
      let anyBind = false
      for (const roleName of resolved.roles) {
        const bind = await bindWarpgateUserRole(data.username, roleName)
        steps.push({
          component: 'warpgate',
          ok: bind.ok,
          detail: bind.detail,
        })
        if (bind.ok) anyBind = true
      }
      if (!anyBind) {
        logger.warn(
          { username: data.username, target: data.target, actor },
          'grant: all warpgate binds failed',
        )
      }
    }
  } else {
    steps.push({
      component: 'warpgate',
      ok: false,
      detail: 'Warpgate admin não configurado (WARPGATE_ADMIN_PASSWORD)',
    })
  }

  // 2) Orchestration best-effort (mock may return ok without effect)
  try {
    const { status, data: body, text } = await orchPost(
      '/orchestration/v1/access/grant',
      {
        username: data.username,
        target: data.target,
        ttl: data.ttl || '8h',
      },
    )
    const st = String(body.status || '')
    steps.push({
      component: 'orchestration',
      ok:
        status >= 200 &&
        status < 300 &&
        (st === 'ok' || st === 'partial' || !st),
      detail:
        status >= 200 && status < 300
          ? [st || 'ok', ...(Array.isArray(body.steps) ? body.steps : [])]
              .join(' · ')
              .slice(0, 400)
          : `HTTP ${status}: ${text.slice(0, 160)}`,
    })
  } catch (e) {
    steps.push({
      component: 'orchestration',
      ok: false,
      detail: (e as Error).message,
    })
  }

  try {
    let object = `connection:${data.target}`
    if (openFgaEnabled()) {
      const { listSites } = await import('./sites')
      const site = (await listSites()).find((candidate) =>
        candidate.targets?.some((target) => target.nome === data.target),
      )
      if (!site) throw new Error(`Target não encontrado no catálogo: ${data.target}`)
      object = openFgaConnectionObject(site.slug, data.target)
    }
    const subject = `user:${data.identity_id || data.username}`
    grantSubject = subject
    grantObject = object
    await writeOpenFgaGrant({
      user: subject,
      relation: 'connect',
      object,
    })
    steps.push({ component: 'openfga', ok: true, detail: 'grant materialized' })
  } catch (e) {
    steps.push({ component: 'openfga', ok: false, detail: (e as Error).message })
  }

  const warpgateOk = steps.some((x) => x.component === 'warpgate' && x.ok)
  const openFgaOk = steps.every((x) => x.component !== 'openfga' || x.ok)
  if (warpgateOk && openFgaOk) {
    try {
      createAccessGrant({
        grant_id: randomUUID(),
        principal: data.username,
        identity_id: data.identity_id,
        target: data.target,
        role: data.role,
        expires_at: expiresAt,
        subject: grantSubject,
        object: grantObject,
      })
      steps.push({ component: 'grant_expiry', ok: true, detail: `expires ${expiresAt}` })
    } catch (e) {
      steps.push({ component: 'grant_expiry', ok: false, detail: (e as Error).message })
    }
  }

  // Success = Warpgate bind OK (critical path). Orch alone is not enough.
  const ok = steps.some((x) => x.component === 'warpgate' && x.ok) &&
    steps.every((x) => !['openfga', 'grant_expiry'].includes(x.component) || x.ok)
  recordActivity(
    'POST',
    `/archgate/persons/${encodeURIComponent(data.username)}/grant`,
    actor,
    ok ? 'success' : 'error',
    undefined,
    {
      target: data.target,
      ttl: data.ttl || '8h',
      expires_at: expiresAt,
      steps: steps.map((x) => `${x.component}:${x.ok ? 'ok' : 'fail'}`).join(','),
    },
  )
  logger.info(
    { username: data.username, target: data.target, actor, ok },
    'grant target',
  )

  return {
    ok,
    username: data.username,
    target: data.target,
    steps,
    message: ok
      ? `Grant ${data.target} → ${data.username} (Warpgate role bound)`
      : `Grant falhou para ${data.username}: confira target aplicado e role WG`,
  }
})

/** Grant target access: Warpgate user↔role (live) + orch best-effort. */
export const grantPersonTargetFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(128),
        identity_id: z.string().min(1).max(256).optional(),
        target: z.string().min(1).max(128),
        /** Optional Warpgate role name (skip auto-resolve). */
        role: z.string().max(128).optional(),
        ttl: z.string().max(32).optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })

  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['persons:update', 'gateways:manage', 'system:admin'],
      'persons:update',
    )
    await assertPrincipalTenantAccess(data.username, s)
    const { listSites } = await import('./sites')
    const site = (await listSites()).find((candidate) =>
      candidate.targets?.some((target) => target.nome === data.target),
    )
    if (!site) throw new Error(`Target não encontrado no catálogo: ${data.target}`)
    assertSiteTenantAccess(site, s)
    if (data.role) {
      const allowedRoles = new Set([
        ...(site.warpgate_roles || []),
        ...(site.targets?.find((target) => target.nome === data.target)?.roles || []),
      ])
      if (!allowedRoles.has(data.role)) throw new Error('Forbidden: role fora do catálogo do target')
    }
    return runGrantPersonTarget({ ...data, identity_id: data.identity_id || undefined }, sessionActor(s))
  })

/** Read the console-owned grant inventory for one person, within tenant scope. */
export const listPersonAccessGrantsFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ username: z.string().min(1).max(128), limit: z.number().int().min(1).max(100).optional(), offset: z.number().int().min(0).optional() }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['persons:read', 'persons:update', 'system:admin'], 'persons:read')
    await assertPrincipalTenantAccess(data.username, s)
    return listAccessGrantsForPrincipal(data.username, data.limit, data.offset)
  })

export const revokePersonAccessGrantFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ grant_id: z.string().min(1).max(128), username: z.string().min(1).max(128) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['persons:update', 'gateways:manage', 'system:admin'], 'persons:update')
    await assertPrincipalTenantAccess(data.username, s)
    const grant = getAccessGrant(data.grant_id)
    if (!grant || grant.principal !== data.username) throw new Error('Grant não encontrado')
    if (openFgaEnabled() && (!grant.subject || !grant.object)) {
      throw new Error('Grant legado sem tupla exata; use o offboarding completo para revogar')
    }
    if (grant.subject && grant.object) {
      const { deleteOpenFgaGrant } = await import('./openfga')
      await deleteOpenFgaGrant({ user: grant.subject, relation: 'connect', object: grant.object })
    }
    revokeAccessGrant(grant.grant_id)
    recordActivity('POST', `/archgate/persons/${encodeURIComponent(data.username)}/grant/${grant.grant_id}/revoke`, sessionActor(s), 'success', undefined, { target: grant.target })
    return {
      ok: true,
      grant_id: grant.grant_id,
      warning: grant.role
        ? 'A role Warpgate compartilhada permanece; use offboarding completo para removê-la.'
        : undefined,
    }
  })
