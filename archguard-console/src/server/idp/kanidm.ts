// Kanidm adapter for the identity-admin port.
//
// This is the behaviour the console shipped with, moved behind the port
// unchanged so staging keeps working while ArchGuard is rolled out.

import { logger } from '../logger'
import type { AdminStep, EnsureGroupResult, IdentityAdmin } from './types'

function baseUrl(): string {
  return (process.env.ARCHGUARD_ID_URL || 'https://localhost:8443').replace(
    /\/$/,
    '',
  )
}

function saToken(): string {
  return process.env.ARCHGUARD_SA_TOKEN || ''
}

async function call(
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; text: string }> {
  const res = await fetch(`${baseUrl()}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${saToken()}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  return { status: res.status, text: await res.text() }
}

export const kanidmAdmin: IdentityAdmin = {
  kind: 'kanidm',

  configured(): boolean {
    return Boolean(baseUrl() && saToken())
  },

  async ensureGroup(name, description): Promise<EnsureGroupResult> {
    const group = name.trim()
    if (!group) return { name, action: 'error', error: 'empty group name' }
    if (!this.configured()) {
      logger.warn('kanidm ensureGroup: SA not configured — skip')
      return { name: group, action: 'skipped', error: 'kanidm SA not configured' }
    }

    try {
      const get = await call('GET', `/v1/group/${encodeURIComponent(group)}`)
      if (get.status >= 200 && get.status < 300) {
        return { name: group, action: 'exists' }
      }
      if (get.status !== 404) {
        return {
          name: group,
          action: 'error',
          error: `GET group HTTP ${get.status}: ${get.text.slice(0, 200)}`,
        }
      }

      const create = await call('POST', '/v1/group', {
        attrs: {
          name: [group],
          description: [description || `ArchGate tenant group ${group}`],
        },
      })
      if (create.status >= 200 && create.status < 300) {
        logger.info({ group }, 'kanidm group created')
        return { name: group, action: 'created' }
      }
      if (create.status === 409 || create.text.toLowerCase().includes('already')) {
        return { name: group, action: 'exists' }
      }
      return {
        name: group,
        action: 'error',
        error: `POST group HTTP ${create.status}: ${create.text.slice(0, 200)}`,
      }
    } catch (e) {
      return { name: group, action: 'error', error: (e as Error).message }
    }
  },

  async addUserToGroup(username, group): Promise<AdminStep> {
    if (!this.configured()) {
      return { ok: false, detail: 'SA token missing' }
    }
    try {
      await this.ensureGroup(group)
      const res = await call(
        'POST',
        `/v1/group/${encodeURIComponent(group)}/_attr/member`,
        { values: [username] },
      )
      if (res.status >= 200 && res.status < 300) {
        return { ok: true, detail: `member → ${group}` }
      }
      if (res.status === 409 || res.text.toLowerCase().includes('already')) {
        return { ok: true, detail: `already in ${group}` }
      }
      return {
        ok: false,
        detail: `HTTP ${res.status}: ${res.text.slice(0, 160)}`,
      }
    } catch (e) {
      return { ok: false, detail: (e as Error).message }
    }
  },

  async disableUser(username): Promise<AdminStep> {
    if (!this.configured()) {
      return { ok: false, detail: 'SA token missing' }
    }
    const id = encodeURIComponent(username)
    const past = '1970-01-01T00:00:00+00:00'
    try {
      // Soft disable via account_expire in the past — preferred over delete so
      // the audit trail still resolves the subject.
      const exp = await call('POST', `/v1/person/${id}/_attr/account_expire`, {
        values: [past],
      })
      if (exp.status >= 200 && exp.status < 300) {
        return { ok: true, detail: 'account_expire set (login blocked)' }
      }
      // Some Kanidm versions expect PUT on the attribute.
      const put = await call('PUT', `/v1/person/${id}/_attr/account_expire`, {
        values: [past],
      })
      if (put.status >= 200 && put.status < 300) {
        return { ok: true, detail: 'account_expire set (PUT)' }
      }
      return {
        ok: false,
        detail: `expire HTTP ${exp.status}: ${exp.text.slice(0, 180)}`,
      }
    } catch (e) {
      return { ok: false, detail: (e as Error).message }
    }
  },
}
