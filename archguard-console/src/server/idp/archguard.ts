// ArchGuard (Casdoor fork) adapter for the identity-admin port.
//
// Differences from Kanidm that shape this file:
//
// - Objects are addressed as `<organization>/<name>`, not by bare name.
// - Group membership lives on the USER (`user.groups`), not on the group, so a
//   bind is read-modify-write against the user. `?columns=groups` keeps the
//   write narrow so a concurrent edit of another field is not clobbered.
// - Disabling is `isForbidden`, not an expiry timestamp.
// - The API answers HTTP 200 with `{"status":"error"}` in the body, so the HTTP
//   status alone never means success.
// - Machine access uses a short-lived bearer from the client_credentials grant.
//   Deliberately not a non-expiring accessKey: a static long-lived secret is
//   what the token-TTL policy exists to avoid. The token is cached and renewed
//   on 401.

import { getCachedAuth, invalidateAuthCache } from '../http-integration-client'
import { logger } from '../logger'
import type { AdminStep, EnsureGroupResult, IdentityAdmin } from './types'

const AUTH_CACHE_KEY = 'archguard-idp-sa'

function baseUrl(): string {
  return (process.env.ARCHGUARD_ID_URL || '').replace(/\/$/, '')
}

function org(): string {
  return process.env.ARCHGUARD_ORG || 'archgate'
}

function clientId(): string {
  return process.env.ARCHGUARD_SA_CLIENT_ID || ''
}

function clientSecret(): string {
  return process.env.ARCHGUARD_SA_CLIENT_SECRET || ''
}

/** `<org>/<name>` — how ArchGuard addresses users and groups. */
function qualify(name: string): string {
  return `${org()}/${name}`
}

type Envelope<T = unknown> = { status?: string; msg?: string; data?: T }

function ok(env: Envelope): boolean {
  return env.status === 'ok'
}

/**
 * Machine bearer via client_credentials. Cached slightly under the token's own
 * lifetime; `invalidate()` forces a fresh grant after a 401.
 */
async function bearer(): Promise<string> {
  const entry = await getCachedAuth(AUTH_CACHE_KEY, 55 * 60 * 1000, async () => {
    const body = new URLSearchParams({
      grant_type: 'client_credentials',
      client_id: clientId(),
      client_secret: clientSecret(),
    })
    const res = await fetch(`${baseUrl()}/api/login/oauth/access_token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    })
    const text = await res.text()
    if (!res.ok) {
      throw new Error(`client_credentials HTTP ${res.status}: ${text.slice(0, 200)}`)
    }
    const parsed = JSON.parse(text) as {
      access_token?: string
      expires_in?: number
      error?: string
    }
    if (!parsed.access_token) {
      throw new Error(`client_credentials: no access_token (${parsed.error || text.slice(0, 120)})`)
    }
    return { value: parsed.access_token }
  })
  return entry.value
}

async function call<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
  retryOn401 = true,
): Promise<{ status: number; env: Envelope<T>; text: string }> {
  const token = await bearer()
  const res = await fetch(`${baseUrl()}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()

  if (res.status === 401 && retryOn401) {
    invalidateAuthCache(AUTH_CACHE_KEY)
    return call<T>(method, path, body, false)
  }

  let env: Envelope<T> = {}
  try {
    env = text ? (JSON.parse(text) as Envelope<T>) : {}
  } catch {
    env = { status: 'error', msg: text.slice(0, 200) }
  }
  return { status: res.status, env, text }
}

type CasdoorUser = {
  owner: string
  name: string
  groups?: string[]
  isForbidden?: boolean
  [key: string]: unknown
}

async function getUser(username: string): Promise<CasdoorUser | null> {
  const { env } = await call<CasdoorUser>(
    'GET',
    `/api/get-user?id=${encodeURIComponent(qualify(username))}`,
  )
  if (!ok(env) || !env.data) return null
  return env.data
}

export const archguardAdmin: IdentityAdmin = {
  kind: 'archguard',

  configured(): boolean {
    return Boolean(baseUrl() && clientId() && clientSecret())
  },

  async ensureGroup(name, description): Promise<EnsureGroupResult> {
    const group = name.trim()
    if (!group) return { name, action: 'error', error: 'empty group name' }
    if (!this.configured()) {
      logger.warn('archguard ensureGroup: SA not configured — skip')
      return {
        name: group,
        action: 'skipped',
        error: 'archguard SA not configured',
      }
    }

    try {
      const get = await call(
        'GET',
        `/api/get-group?id=${encodeURIComponent(qualify(group))}`,
      )
      if (ok(get.env) && get.env.data) {
        return { name: group, action: 'exists' }
      }

      const create = await call('POST', '/api/add-group', {
        owner: org(),
        name: group,
        displayName: description || `ArchGate tenant group ${group}`,
        type: 'Virtual',
        isTopGroup: true,
      })
      if (ok(create.env)) {
        logger.info({ group }, 'archguard group created')
        return { name: group, action: 'created' }
      }
      // Lost a race against another writer.
      if ((create.env.msg || '').toLowerCase().includes('exist')) {
        return { name: group, action: 'exists' }
      }
      return {
        name: group,
        action: 'error',
        error: `add-group: ${create.env.msg || create.text.slice(0, 200)}`,
      }
    } catch (e) {
      return { name: group, action: 'error', error: (e as Error).message }
    }
  },

  async addUserToGroup(username, group): Promise<AdminStep> {
    if (!this.configured()) {
      return { ok: false, detail: 'archguard SA not configured' }
    }
    try {
      const ensured = await this.ensureGroup(group)
      if (ensured.action === 'error') {
        return { ok: false, detail: `group: ${ensured.error}` }
      }

      const user = await getUser(username)
      if (!user) {
        return { ok: false, detail: `user ${username} not found` }
      }

      const qualified = qualify(group)
      const current = Array.isArray(user.groups) ? user.groups : []
      if (current.includes(qualified)) {
        return { ok: true, detail: `already in ${group}` }
      }

      // Narrow write: only the groups column, so a concurrent edit elsewhere
      // on the user is not overwritten by this read-modify-write.
      const res = await call(
        'POST',
        `/api/update-user?id=${encodeURIComponent(qualify(username))}&columns=groups`,
        { ...user, groups: [...current, qualified] },
      )
      if (ok(res.env)) {
        return { ok: true, detail: `member → ${group}` }
      }
      return {
        ok: false,
        detail: `update-user: ${res.env.msg || res.text.slice(0, 160)}`,
      }
    } catch (e) {
      return { ok: false, detail: (e as Error).message }
    }
  },

  async disableUser(username): Promise<AdminStep> {
    if (!this.configured()) {
      return { ok: false, detail: 'archguard SA not configured' }
    }
    try {
      const user = await getUser(username)
      if (!user) {
        return { ok: false, detail: `user ${username} not found` }
      }
      if (user.isForbidden === true) {
        return { ok: true, detail: 'already forbidden (login blocked)' }
      }

      const res = await call(
        'POST',
        `/api/update-user?id=${encodeURIComponent(qualify(username))}&columns=is_forbidden`,
        { ...user, isForbidden: true },
      )
      if (ok(res.env)) {
        return { ok: true, detail: 'isForbidden set (login blocked)' }
      }
      return {
        ok: false,
        detail: `update-user: ${res.env.msg || res.text.slice(0, 180)}`,
      }
    } catch (e) {
      return { ok: false, detail: (e as Error).message }
    }
  },
}
