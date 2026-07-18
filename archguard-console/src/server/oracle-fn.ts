// W-C4 — Oracle credentials via oracle-proxy (+ optional OpenBao database role)

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { randomBytes } from 'node:crypto'
import { recordActivity } from './activity-log'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { logger } from './logger'
import { integrationFetch } from './http-integration-client'
import {
  openbaoTokenConfigured,
  openbaoAddr,
} from './openbao-proxy'
import { writeTargetSecret } from './openbao-proxy'

const ORACLE_PROXY_URL = (
  process.env.ORACLE_PROXY_URL ||
  process.env.ARCHGATE_ORACLE_PROXY_URL ||
  'http://archgate-oracle-proxy:9040'
).replace(/\/$/, '')

const OPENBAO_ORACLE_ROLE =
  process.env.OPENBAO_ORACLE_ROLE || 'oracle-readonly'

async function proxyApi(
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; data: Record<string, unknown>; text: string }> {
  const res = await integrationFetch(`${ORACLE_PROXY_URL}${path}`, {
    method,
    integration: 'oracle-proxy',
    headers: { 'Content-Type': 'application/json' },
    body: body !== undefined ? JSON.stringify(body) : undefined,
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

function genUsername(prefix = 'v'): string {
  // Oracle identifier ≤ 30
  const rnd = randomBytes(4).toString('hex')
  const u = `${prefix}_${rnd}`
  return u.slice(0, 30)
}

function genPassword(): string {
  // Avoid Oracle-problematic chars
  const raw = randomBytes(18).toString('base64url')
  return `Ag${raw.replace(/[^a-zA-Z0-9]/g, 'x').slice(0, 20)}!`
}

export const getOracleStatusFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['secrets:read', 'secrets:manage', 'system:admin'],
      'secrets:read',
    )

    let proxy: {
      ok: boolean
      url: string
      health?: unknown
      error?: string
    } = { ok: false, url: ORACLE_PROXY_URL }

    try {
      const { status, data, text } = await proxyApi(
        'GET',
        '/actuator/health',
      )
      proxy = {
        ok: status >= 200 && status < 300,
        url: ORACLE_PROXY_URL,
        health: data,
        error: status >= 400 ? text.slice(0, 120) : undefined,
      }
    } catch (e) {
      proxy = {
        ok: false,
        url: ORACLE_PROXY_URL,
        error: (e as Error).message,
      }
    }

    let openbaoRole: {
      configured: boolean
      role: string
      reachable?: boolean
      error?: string
    } = {
      configured: openbaoTokenConfigured(),
      role: OPENBAO_ORACLE_ROLE,
    }

    if (openbaoTokenConfigured()) {
      try {
        // Probe role exists: LIST/GET database/roles/:name varies; try read
        const res = await integrationFetch(
          `${openbaoAddr()}/v1/database/roles/${encodeURIComponent(OPENBAO_ORACLE_ROLE)}`,
          {
            method: 'GET',
            integration: 'openbao',
            headers: {
              'X-Vault-Token':
                process.env.OPENBAO_APP_TOKEN ||
                process.env.OPENBAO_TOKEN ||
                process.env.OPENBAO_ROOT_TOKEN ||
                '',
            },
          },
        )
        openbaoRole.reachable = res.status >= 200 && res.status < 300
        if (!openbaoRole.reachable) {
          openbaoRole.error = `role HTTP ${res.status} (plugin/role may not be registered)`
        }
      } catch (e) {
        openbaoRole.reachable = false
        openbaoRole.error = (e as Error).message
      }
    }

    return {
      proxy,
      openbao: openbaoRole,
      mock_hint:
        'Com ORACLE_PROXY_MOCK=true o proxy só simula CREATE/DROP USER (lab).',
    }
  },
)

/**
 * Issue dynamic Oracle credential:
 * 1) Prefer OpenBao database/creds/:role if role reachable
 * 2) Else direct oracle-proxy create (mock or JDBC)
 */
export const issueOracleCredentialFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        /** Prefer openbao | proxy */
        mode: z.enum(['auto', 'openbao', 'proxy']).default('auto'),
        username_prefix: z.string().max(8).optional(),
        /** Store copy in OpenBao KV for operator retrieval */
        store_in_kv: z.boolean().default(true),
        site_slug: z.string().optional(),
        ttl_hint: z.string().optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['secrets:manage', 'system:admin'],
      'secrets:manage',
    )
    const actor = sessionActor(s)

    // --- OpenBao dynamic ---
    const tryOpenbao =
      data.mode === 'openbao' ||
      (data.mode === 'auto' && openbaoTokenConfigured())

    if (tryOpenbao && data.mode !== 'proxy') {
      try {
        const token =
          process.env.OPENBAO_APP_TOKEN ||
          process.env.OPENBAO_TOKEN ||
          process.env.OPENBAO_ROOT_TOKEN ||
          ''
        const res = await integrationFetch(
          `${openbaoAddr()}/v1/database/creds/${encodeURIComponent(OPENBAO_ORACLE_ROLE)}`,
          {
            method: 'GET',
            integration: 'openbao',
            headers: { 'X-Vault-Token': token },
          },
        )
        const text = await res.text()
        if (res.status >= 200 && res.status < 300) {
          const json = JSON.parse(text) as {
            lease_id?: string
            lease_duration?: number
            data?: { username?: string; password?: string }
          }
          const username = json.data?.username || ''
          const password = json.data?.password || ''
          let secret_ref: string | undefined
          if (data.store_in_kv && username && password) {
            const w = await writeTargetSecret({
              name: `oracle_${username}`,
              password,
              username,
              extra: {
                source: 'openbao_database',
                lease_id: json.lease_id || '',
              },
            })
            secret_ref = w.secret_ref
          }
          recordActivity(
            'POST',
            '/archgate/oracle/creds',
            actor,
            'success',
            undefined,
            { mode: 'openbao', username, lease_id: json.lease_id },
          )
          return {
            ok: true,
            mode: 'openbao' as const,
            username,
            password,
            lease_id: json.lease_id,
            lease_duration: json.lease_duration,
            secret_ref,
            message: `Credencial Oracle emitida via OpenBao role ${OPENBAO_ORACLE_ROLE}`,
          }
        }
        if (data.mode === 'openbao') {
          throw new Error(
            `OpenBao database/creds failed HTTP ${res.status}: ${text.slice(0, 200)}`,
          )
        }
        // auto → fall through to proxy
        logger.info(
          { status: res.status },
          'oracle openbao role unavailable, using proxy',
        )
      } catch (e) {
        if (data.mode === 'openbao') throw e
        logger.warn({ err: String(e) }, 'oracle openbao path failed')
      }
    }

    // --- Direct proxy ---
    const username = genUsername(data.username_prefix || 'v')
    const password = genPassword()
    const { status, data: body, text } = await proxyApi(
      'POST',
      '/v1/oracle/users',
      {
        username,
        password,
        expiration: data.ttl_hint || undefined,
      },
    )
    if (status >= 400) {
      recordActivity(
        'POST',
        '/archgate/oracle/creds',
        actor,
        'error',
        text.slice(0, 200),
      )
      throw new Error(`oracle-proxy create failed HTTP ${status}: ${text.slice(0, 200)}`)
    }

    let secret_ref: string | undefined
    if (data.store_in_kv) {
      try {
        const w = await writeTargetSecret({
          name: `oracle_${username}`,
          password,
          username,
          extra: {
            source: 'oracle_proxy',
            site: data.site_slug || '',
          },
        })
        secret_ref = w.secret_ref
      } catch (e) {
        logger.warn({ err: String(e) }, 'oracle store_in_kv failed')
      }
    }

    recordActivity(
      'POST',
      '/archgate/oracle/creds',
      actor,
      'success',
      undefined,
      { mode: 'proxy', username },
    )

    return {
      ok: true,
      mode: 'proxy' as const,
      username,
      password,
      lease_id: undefined as string | undefined,
      lease_duration: undefined as number | undefined,
      secret_ref,
      proxy_response: body,
      message:
        'Credencial criada no oracle-proxy (mock ou JDBC). Guarde a senha — exibida só agora.',
    }
  })

export const revokeOracleCredentialFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        username: z.string().min(1).max(30),
        lease_id: z.string().optional(),
        mode: z.enum(['auto', 'openbao', 'proxy']).default('auto'),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['secrets:manage', 'system:admin'], 'secrets:manage')
    const actor = sessionActor(s)
    const steps: { step: string; ok: boolean; detail?: string }[] = []

    if (data.lease_id && openbaoTokenConfigured()) {
      try {
        const token =
          process.env.OPENBAO_APP_TOKEN ||
          process.env.OPENBAO_TOKEN ||
          process.env.OPENBAO_ROOT_TOKEN ||
          ''
        const res = await integrationFetch(
          `${openbaoAddr()}/v1/sys/leases/revoke`,
          {
            method: 'PUT',
            integration: 'openbao',
            headers: {
              'X-Vault-Token': token,
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ lease_id: data.lease_id }),
          },
        )
        steps.push({
          step: 'openbao_revoke_lease',
          ok: res.status >= 200 && res.status < 300,
          detail: `HTTP ${res.status}`,
        })
      } catch (e) {
        steps.push({
          step: 'openbao_revoke_lease',
          ok: false,
          detail: (e as Error).message,
        })
      }
    }

    // Always drop user via proxy (idempotent for mock)
    try {
      const { status, text } = await proxyApi('POST', '/v1/oracle/users/delete', {
        username: data.username,
      })
      steps.push({
        step: 'proxy_delete_user',
        ok: status >= 200 && status < 300,
        detail:
          status >= 200 && status < 300
            ? 'ok'
            : `HTTP ${status}: ${text.slice(0, 120)}`,
      })
    } catch (e) {
      steps.push({
        step: 'proxy_delete_user',
        ok: false,
        detail: (e as Error).message,
      })
    }

    const ok = steps.some((x) => x.ok)
    recordActivity(
      'POST',
      '/archgate/oracle/revoke',
      actor,
      ok ? 'success' : 'error',
      undefined,
      { username: data.username, lease_id: data.lease_id },
    )

    return {
      ok,
      username: data.username,
      steps,
      message: ok
        ? `Credencial ${data.username} revogada`
        : `Falha ao revogar ${data.username}`,
    }
  })
