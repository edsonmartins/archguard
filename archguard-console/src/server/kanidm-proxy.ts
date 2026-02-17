// src/server/kanidm-proxy.ts

import { createServerFn } from '@tanstack/react-start'
import { getCookie } from '@tanstack/react-start/server'
import { decryptSession } from './session'
import type { SessionData } from './auth'

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN!

// Allowed API path prefixes to prevent SSRF
const ALLOWED_PATH_PREFIXES = [
  '/v1/person',
  '/v1/group',
  '/v1/oauth2',
  '/v1/service_account',
  '/v1/domain',
  '/v1/system',
  '/v1/recycle_bin',
  '/status',
]

function isAllowedPath(path: string): boolean {
  // Normalize: remove double slashes, trailing slash
  const normalized = path.replace(/\/+/g, '/').replace(/\/$/, '')

  // Must start with an allowed prefix
  return ALLOWED_PATH_PREFIXES.some(
    (prefix) =>
      normalized === prefix || normalized.startsWith(prefix + '/'),
  )
}

function isAuthenticated(): boolean {
  try {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) return false
    const session = decryptSession<SessionData>(sessionCookie)
    return session.isAuthenticated === true
  } catch {
    return false
  }
}

export const kanidmApiFn = createServerFn({ method: 'POST' })
  .inputValidator(
    (data: {
      method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
      path: string
      body?: unknown
    }) => data,
  )
  .handler(async ({ data }) => {
    // Auth check: only authenticated users can use the proxy
    if (!isAuthenticated()) {
      throw new Error('Unauthorized: session required')
    }

    // SSRF prevention: validate path against allowlist
    if (!isAllowedPath(data.path)) {
      throw new Error(`Forbidden path: ${data.path}`)
    }

    const response = await fetch(`${KANIDM_URL}${data.path}`, {
      method: data.method,
      headers: {
        Authorization: `Bearer ${KANIDM_SA_TOKEN}`,
        'Content-Type': 'application/json',
      },
      body: data.body ? JSON.stringify(data.body) : undefined,
    })

    if (!response.ok) {
      const error = await response.text()
      throw new Error(`Kanidm API ${response.status}: ${error}`)
    }

    const text = await response.text()
    return text ? JSON.parse(text) : null
  })
