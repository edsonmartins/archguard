// src/server/kanidm-proxy.ts

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { recordActivity, getActor } from './activity-log'
import { logger } from './logger'
import { enforceRateLimit } from './rate-limit'
import { requireSession } from './session-guard'

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN!
const PROXY_LIMIT = 60
const PROXY_WINDOW_MS = 60 * 1000

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

const proxyRequestSchema = z.object({
  method: z.enum(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']),
  path: z.string().min(1).max(2048).startsWith('/'),
  body: z.unknown().optional(),
})

export function isAllowedPath(path: string): boolean {
  // Normalize: collapse repeated slashes, strip trailing slash.
  const normalized = path.replace(/\/+/g, '/').replace(/\/$/, '')

  // Reject path-traversal segments that would let the request escape the
  // allowlist after the upstream resolves them.
  const segments = normalized.split('/')
  if (segments.some((s) => s === '..' || s === '.')) return false

  // Must start with an allowed prefix.
  return ALLOWED_PATH_PREFIXES.some(
    (prefix) =>
      normalized === prefix || normalized.startsWith(prefix + '/'),
  )
}

export const kanidmApiFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const result = proxyRequestSchema.safeParse(data)
    if (!result.success) {
      throw new Error(`Invalid proxy request: ${result.error.message}`)
    }
    return result.data
  })
  .handler(async ({ data }) => {
    enforceRateLimit('proxy', PROXY_LIMIT, PROXY_WINDOW_MS)

    // Auth check: only authenticated users can use the proxy
    try {
      requireSession()
    } catch {
      logger.warn(
        { method: data.method, path: data.path },
        'proxy: rejected unauthenticated request',
      )
      throw new Error('Unauthorized: session required')
    }

    // SSRF prevention: validate path against allowlist
    if (!isAllowedPath(data.path)) {
      logger.warn(
        { actor: getActor(), method: data.method, path: data.path },
        'proxy: rejected path not in allowlist',
      )
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

    const isMutation = data.method !== 'GET'

    if (!response.ok) {
      const error = await response.text()
      if (isMutation) {
        recordActivity(
          data.method,
          data.path,
          getActor(),
          'error',
          error,
          data.body,
        )
      }
      logger.error(
        {
          actor: getActor(),
          method: data.method,
          path: data.path,
          status: response.status,
        },
        'proxy: kanidm api error',
      )
      throw new Error(`Kanidm API ${response.status}: ${error}`)
    }

    if (isMutation) {
      recordActivity(
        data.method,
        data.path,
        getActor(),
        'success',
        undefined,
        data.body,
      )
    }

    const text = await response.text()
    return text ? JSON.parse(text) : null
  })
