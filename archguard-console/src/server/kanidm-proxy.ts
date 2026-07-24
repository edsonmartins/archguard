// src/server/kanidm-proxy.ts

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import type { Permission } from '@/lib/auth/permissions'
import { recordActivity, getActor } from './activity-log'
import { logger } from './logger'
import { enforceRateLimit } from './rate-limit'
import { requireAnyPerm, requireSession } from './session-guard'

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

/**
 * Map Kanidm proxy method+path → required console permissions.
 * Deny-by-default: unknown shapes require system:admin.
 * Viewers (persons:read only) cannot mutate via the shared SA token.
 */
export function requiredPermsForKanidmProxy(
  method: string,
  path: string,
): Permission[] {
  const normalized = path.replace(/\/+/g, '/').replace(/\/$/, '') || path
  const m = method.toUpperCase()
  const isGet = m === 'GET'

  if (normalized === '/status' || normalized.startsWith('/status/')) {
    return ['persons:read', 'settings:read', 'system:admin']
  }

  // Person credentials / SSH keys
  if (
    normalized.includes('/_credential') ||
    normalized.includes('/_ssh_pubkeys')
  ) {
    return isGet
      ? ['persons:read', 'persons:credentials']
      : ['persons:credentials', 'persons:update', 'system:admin']
  }

  if (
    normalized === '/v1/person' ||
    normalized.startsWith('/v1/person/')
  ) {
    if (isGet) return ['persons:read']
    if (m === 'POST' && normalized === '/v1/person') return ['persons:create']
    if (m === 'DELETE') return ['persons:delete']
    return ['persons:update']
  }

  if (
    normalized === '/v1/group' ||
    normalized.startsWith('/v1/group/')
  ) {
    if (normalized.includes('/_attr/member')) {
      return isGet
        ? ['groups:read', 'groups:members']
        : ['groups:members', 'groups:update', 'system:admin']
    }
    if (isGet) return ['groups:read']
    if (m === 'POST' && normalized === '/v1/group') return ['groups:create']
    if (m === 'DELETE') return ['groups:delete']
    return ['groups:update', 'groups:members', 'system:admin']
  }

  if (
    normalized === '/v1/oauth2' ||
    normalized.startsWith('/v1/oauth2/')
  ) {
    if (
      normalized.includes('_basic_secret') ||
      normalized.includes('_secret')
    ) {
      return isGet
        ? ['oauth2:secrets', 'oauth2:read']
        : ['oauth2:secrets', 'oauth2:update', 'system:admin']
    }
    if (isGet) return ['oauth2:read']
    if (m === 'POST' && normalized === '/v1/oauth2') return ['oauth2:create']
    if (m === 'DELETE') return ['oauth2:delete']
    return ['oauth2:update']
  }

  if (
    normalized === '/v1/service_account' ||
    normalized.startsWith('/v1/service_account/')
  ) {
    if (normalized.includes('_api_token') || normalized.includes('/_token')) {
      return isGet
        ? ['service_accounts:read', 'service_accounts:tokens']
        : ['service_accounts:tokens', 'system:admin']
    }
    if (isGet) return ['service_accounts:read']
    if (m === 'POST' && normalized === '/v1/service_account') {
      return ['service_accounts:create']
    }
    if (m === 'DELETE') return ['service_accounts:delete']
    return ['service_accounts:create', 'system:admin']
  }

  if (
    normalized === '/v1/recycle_bin' ||
    normalized.startsWith('/v1/recycle_bin/')
  ) {
    if (isGet) return ['persons:read', 'persons:delete', 'system:admin']
    return ['persons:delete', 'system:admin']
  }

  if (
    normalized === '/v1/domain' ||
    normalized.startsWith('/v1/domain/') ||
    normalized === '/v1/system' ||
    normalized.startsWith('/v1/system/')
  ) {
    if (isGet) return ['settings:read', 'system:admin']
    return ['settings:update', 'system:admin']
  }

  return ['system:admin']
}

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

    // Auth + RBAC: authenticated session with permission for this method/path.
    // Without this, any logged-in viewer could mutate Kanidm via the SA token.
    let session
    try {
      session = requireSession()
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

    const needed = requiredPermsForKanidmProxy(data.method, data.path)
    try {
      requireAnyPerm(session, needed, needed.join(' | '))
    } catch (e) {
      logger.warn(
        {
          actor: getActor(),
          method: data.method,
          path: data.path,
          needed,
        },
        'proxy: rejected insufficient permissions',
      )
      throw e
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
