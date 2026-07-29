// src/server/rate-limit.ts
//
// Sliding-window rate limiter, in-memory, single-process.
// Sufficient for one-instance deploys; if the console scales horizontally,
// move this to Redis (e.g. ioredis with INCR + PEXPIRE).

import { getRequestIP } from '@tanstack/react-start/server'
import { logger } from './logger'

const buckets = new Map<string, number[]>()

function clientKey(scope: string): string {
  let ip: string | undefined
  try {
    ip = getRequestIP({ xForwardedFor: true })
  } catch {
    ip = undefined
  }
  return `${scope}:${ip || 'unknown'}`
}

// E2E pumps several API calls per test through one IP, which trips the
// proxy rate limit halfway through the suite and starts cascading factory
// timeouts. This has its OWN opt-in flag, set only by the Playwright harness.
//
// It used to piggyback on ARCHGUARD_E2E_LOGIN, but the staging deploy sets
// that flag by default, which silently disabled brute-force protection on a
// public host (audit 2026-07-28, P0-2). The two concerns are now independent.
const E2E_DISABLED = process.env.ARCHGUARD_E2E_DISABLE_RATE_LIMIT === '1'

/**
 * Throws if the caller exceeded `limit` calls in the trailing `windowMs`.
 * Buckets are scoped per IP per `scope` name.
 */
export function enforceRateLimit(
  scope: string,
  limit: number,
  windowMs: number,
): void {
  if (E2E_DISABLED) return
  const key = clientKey(scope)
  const now = Date.now()
  const cutoff = now - windowMs

  const hits = (buckets.get(key) ?? []).filter((ts) => ts > cutoff)
  hits.push(now)
  buckets.set(key, hits)

  if (hits.length > limit) {
    logger.warn(
      { scope, key, hits: hits.length, limit, windowMs },
      'rate-limit: rejected request',
    )
    throw new Error('Too many requests')
  }
}

// Periodic cleanup so the map does not grow unbounded.
const CLEANUP_INTERVAL_MS = 5 * 60 * 1000
const MAX_RETENTION_MS = 60 * 60 * 1000
setInterval(() => {
  const cutoff = Date.now() - MAX_RETENTION_MS
  for (const [key, hits] of buckets) {
    const kept = hits.filter((ts) => ts > cutoff)
    if (kept.length === 0) buckets.delete(key)
    else buckets.set(key, kept)
  }
}, CLEANUP_INTERVAL_MS).unref?.()
