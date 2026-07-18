// W-C2 — One-click person offboarding (AWS-style access revoke)

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
import {
  deleteWarpgateUserByName,
  warpgateConfigured,
} from './warpgate-proxy'

const KANIDM_URL = (
  process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
).replace(/\/$/, '')
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN || ''

/** Prefer internal compose service; host.docker.internal for agent-style */
const ORCH_URL = (
  process.env.ORCHESTRATION_URL ||
  process.env.ARCHGATE_ORCHESTRATION_URL ||
  'http://archgate-orchestration:8090'
).replace(/\/$/, '')

export type OffboardStep = {
  component: string
  ok: boolean
  detail?: string
}

async function kanidmSa(
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; text: string }> {
  if (!KANIDM_SA_TOKEN) {
    throw new Error('ARCHGUARD_SA_TOKEN ausente')
  }
  const res = await fetch(`${KANIDM_URL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${KANIDM_SA_TOKEN}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  return { status: res.status, text: await res.text() }
}

/**
 * Expire Kanidm person (soft disable — preferred over hard delete for audit).
 * Uses account_expire in the past so login is denied immediately.
 */
async function expireKanidmPerson(username: string): Promise<OffboardStep> {
  const id = encodeURIComponent(username)
  try {
    // Prefer name; Kanidm accepts spn/name on many installs
    const exp = await kanidmSa(
      'POST',
      `/v1/person/${id}/_attr/account_expire`,
      { values: ['1970-01-01T00:00:00+00:00'] },
    )
    if (exp.status >= 200 && exp.status < 300) {
      return {
        component: 'kanidm',
        ok: true,
        detail: 'account_expire set (login blocked)',
      }
    }
    // Some versions use PUT on attr
    const exp2 = await kanidmSa(
      'PUT',
      `/v1/person/${id}/_attr/account_expire`,
      { values: ['1970-01-01T00:00:00+00:00'] },
    )
    if (exp2.status >= 200 && exp2.status < 300) {
      return {
        component: 'kanidm',
        ok: true,
        detail: 'account_expire set (PUT)',
      }
    }
    return {
      component: 'kanidm',
      ok: false,
      detail: `expire HTTP ${exp.status}: ${exp.text.slice(0, 180)}`,
    }
  } catch (e) {
    return { component: 'kanidm', ok: false, detail: (e as Error).message }
  }
}

async function callOrchestrationRevoke(
  username: string,
  reason: string,
): Promise<OffboardStep> {
  try {
    const res = await integrationFetch(
      `${ORCH_URL}/orchestration/v1/users/revoke`,
      {
        method: 'POST',
        integration: 'orchestration',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, reason }),
      },
    )
    const text = await res.text()
    let data: { status?: string; steps?: string[]; details?: Record<string, string> } =
      {}
    try {
      data = text ? JSON.parse(text) : {}
    } catch {
      data = {}
    }
    if (res.status >= 200 && res.status < 300) {
      return {
        component: 'orchestration',
        ok: data.status === 'ok' || data.status === 'partial' || !data.status,
        detail: [
          data.status || 'ok',
          ...(data.steps || []),
          data.details?.errors,
        ]
          .filter(Boolean)
          .join(' · ')
          .slice(0, 400),
      }
    }
    return {
      component: 'orchestration',
      ok: false,
      detail: `HTTP ${res.status}: ${text.slice(0, 200)}`,
    }
  } catch (e) {
    return {
      component: 'orchestration',
      ok: false,
      detail: (e as Error).message,
    }
  }
}

/**
 * One-shot offboarding:
 * 1. Orchestration service (mock or live multi-adapter)
 * 2. Kanidm expire (real access kill — always attempted)
 * 3. Warpgate user remove (best-effort if configured)
 */
export const revokePersonAccessFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(128),
        person_id: z.string().optional(),
        reason: z.string().max(500).optional(),
        /** If true, skip orch and only direct Kanidm/WG */
        direct_only: z.boolean().optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['persons:update', 'persons:delete', 'system:admin'],
      'persons:update',
    )
    const actor = sessionActor(s)
    const reason = data.reason || `offboarding by ${actor}`
    const username = data.username.trim()
    const steps: OffboardStep[] = []

    if (!data.direct_only) {
      steps.push(await callOrchestrationRevoke(username, reason))
    }

    // Always enforce IdP block even if orch is mock-only
    steps.push(await expireKanidmPerson(username))

    if (warpgateConfigured()) {
      try {
        const r = await deleteWarpgateUserByName(username)
        steps.push({
          component: 'warpgate',
          ok: r.ok,
          detail: r.detail,
        })
      } catch (e) {
        steps.push({
          component: 'warpgate',
          ok: false,
          detail: (e as Error).message,
        })
      }
    } else {
      steps.push({
        component: 'warpgate',
        ok: true,
        detail: 'skipped (not configured)',
      })
    }

    const kanidmOk = steps.find((x) => x.component === 'kanidm')?.ok
    const allOk = steps.every((x) => x.ok)
    const criticalOk = !!kanidmOk

    recordActivity(
      'POST',
      `/archgate/persons/${encodeURIComponent(username)}/revoke`,
      actor,
      criticalOk ? 'success' : 'error',
      criticalOk ? undefined : 'kanidm expire failed',
      {
        reason,
        steps: steps.map((x) => `${x.component}:${x.ok ? 'ok' : 'fail'}`).join(','),
      },
    )

    logger.info(
      { username, actor, criticalOk, steps: steps.length },
      'person offboarding',
    )

    return {
      ok: criticalOk,
      all_ok: allOk,
      username,
      reason,
      steps,
      message: criticalOk
        ? `Acesso revogado para ${username} (login Kanidm bloqueado)`
        : `Falha ao revogar ${username} — ver steps`,
    }
  })
