// W-C2 — provision + grant person access (orchestration + evidence)

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

/** Grant target/role access (orch warpgate + openbao adapters). */
export const grantPersonTargetFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(128),
        target: z.string().min(1).max(128),
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
    const actor = sessionActor(s)
    const steps: LifecycleStep[] = []

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

    const ok = steps.some((x) => x.ok)
    recordActivity(
      'POST',
      `/archgate/persons/${encodeURIComponent(data.username)}/grant`,
      actor,
      ok ? 'success' : 'error',
      undefined,
      { target: data.target, ttl: data.ttl || '8h' },
    )

    return {
      ok,
      username: data.username,
      target: data.target,
      steps,
      message: ok
        ? `Grant ${data.target} → ${data.username}`
        : `Grant falhou para ${data.username}`,
    }
  })
