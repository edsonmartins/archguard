// Shared HTTP helpers for outbound control-plane integrations
// (Warpgate, Guacamole, OpenBao, Mentors Axis).
// - timeouts via AbortController
// - short-lived auth cache (cookies / tokens) to avoid login storms

import { logger } from './logger'

export type IntegrationAuthCacheEntry = {
  value: string
  expiresAt: number
  /** Extra bag (e.g. Guacamole dataSource). */
  meta?: Record<string, string>
}

const authCaches = new Map<string, IntegrationAuthCacheEntry>()

const DEFAULT_TIMEOUT_MS = 15_000

/**
 * Get or refresh a cached auth artifact (session cookie, token, …).
 * On loader failure the previous entry (if any) is discarded.
 */
export async function getCachedAuth(
  key: string,
  ttlMs: number,
  loader: () => Promise<{ value: string; meta?: Record<string, string> }>,
): Promise<IntegrationAuthCacheEntry> {
  const hit = authCaches.get(key)
  if (hit && hit.expiresAt > Date.now()) {
    return hit
  }
  try {
    const loaded = await loader()
    const entry: IntegrationAuthCacheEntry = {
      value: loaded.value,
      meta: loaded.meta,
      expiresAt: Date.now() + Math.max(1_000, ttlMs),
    }
    authCaches.set(key, entry)
    return entry
  } catch (e) {
    authCaches.delete(key)
    throw e
  }
}

export function invalidateAuthCache(key?: string): void {
  if (key) {
    authCaches.delete(key)
    return
  }
  authCaches.clear()
}

export type IntegrationFetchInit = RequestInit & {
  /** Abort after this many ms (default 15s). */
  timeoutMs?: number
  /** Logical integration name for logs. */
  integration?: string
}

/**
 * fetch with timeout. Does not retry; callers decide auth-retry policy.
 */
export async function integrationFetch(
  url: string,
  init: IntegrationFetchInit = {},
): Promise<Response> {
  const { timeoutMs = DEFAULT_TIMEOUT_MS, integration, ...rest } = init
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  const started = Date.now()
  try {
    const res = await fetch(url, {
      ...rest,
      signal: rest.signal ?? controller.signal,
    })
    if (integration && res.status >= 500) {
      logger.warn(
        {
          integration,
          url,
          status: res.status,
          ms: Date.now() - started,
        },
        'integration upstream 5xx',
      )
    }
    return res
  } catch (e) {
    if ((e as Error).name === 'AbortError') {
      throw new Error(
        `${integration || 'integration'} timeout after ${timeoutMs}ms: ${url}`,
      )
    }
    throw e
  } finally {
    clearTimeout(timer)
  }
}

export async function integrationJson<T = unknown>(
  url: string,
  init: IntegrationFetchInit = {},
): Promise<{ status: number; data: T; text: string }> {
  const res = await integrationFetch(url, init)
  const text = await res.text()
  let data: T
  try {
    data = text ? (JSON.parse(text) as T) : ({} as T)
  } catch {
    data = { raw: text } as T
  }
  return { status: res.status, data, text }
}
