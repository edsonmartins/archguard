// W-C2 — provision + grant person access (orchestration + Warpgate live + evidence)
//
// Warpgate/sites are loaded via dynamic import inside handlers so the client
// Vite graph never pulls node:https / better-sqlite3 (see docker build).

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { recordActivity } from './activity-log'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { logger } from './logger'
import { integrationFetch } from './http-integration-client'
import { ensureKanidmGroup } from './kanidm-admin'

const ORCH_URL = (
  process.env.ORCHESTRATION_URL ||
  process.env.ARCHGATE_ORCHESTRATION_URL ||
  'http://archgate-orchestration:8090'
).replace(/\/$/, '')

const KANIDM_URL = (
  process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
).replace(/\/$/, '')
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN || ''

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

async function kanidmAddToGroup(
  username: string,
  group: string,
): Promise<LifecycleStep> {
  if (!KANIDM_SA_TOKEN) {
    return { component: 'kanidm_group', ok: false, detail: 'SA token missing' }
  }
  try {
    await ensureKanidmGroup(group)
    const res = await fetch(
      `${KANIDM_URL}/v1/group/${encodeURIComponent(group)}/_attr/member`,
      {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${KANIDM_SA_TOKEN}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ values: [username] }),
      },
    )
    const text = await res.text()
    if (res.status >= 200 && res.status < 300) {
      return {
        component: 'kanidm_group',
        ok: true,
        detail: `member → ${group}`,
      }
    }
    // already member / alternate shape
    if (res.status === 409 || text.toLowerCase().includes('already')) {
      return {
        component: 'kanidm_group',
        ok: true,
        detail: `already in ${group}`,
      }
    }
    return {
      component: 'kanidm_group',
      ok: false,
      detail: `HTTP ${res.status}: ${text.slice(0, 160)}`,
    }
  } catch (e) {
    return {
      component: 'kanidm_group',
      ok: false,
      detail: (e as Error).message,
    }
  }
}

/**
 * Ensure person exists in platform adapters (orch) + tenant/groups in Kanidm.
 * Person must already exist in Kanidm (created via console identities).
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

    // Direct Kanidm membership (real path even if orch mock)
    for (const g of groups) {
      steps.push(await kanidmAddToGroup(data.username, g))
    }

    const critical = steps.some(
      (x) => x.component === 'kanidm_group' && x.ok,
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
export async function resolveGrantRoles(
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
}

export type GrantPersonTargetInput = {
  username: string
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

/**
 * Core grant logic (Warpgate live bind + orch best-effort).
 * Used by Manager UI server-fn and lab smoke API.
 */
export async function runGrantPersonTarget(
  data: GrantPersonTargetInput,
  actor: string,
): Promise<GrantPersonTargetResult> {
  const steps: LifecycleStep[] = []

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

  // Success = Warpgate bind OK (critical path). Orch alone is not enough.
  const ok = steps.some((x) => x.component === 'warpgate' && x.ok)
  recordActivity(
    'POST',
    `/archgate/persons/${encodeURIComponent(data.username)}/grant`,
    actor,
    ok ? 'success' : 'error',
    undefined,
    {
      target: data.target,
      ttl: data.ttl || '8h',
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
}

/** Grant target access: Warpgate user↔role (live) + orch best-effort. */
export const grantPersonTargetFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(128),
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
    return runGrantPersonTarget(data, sessionActor(s))
  })
