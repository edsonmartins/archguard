import { closeRustGuacSession } from './rustguac-proxy'
import { closeBrokerSession, getBrokerSession, getLatestBrokerReconciliationRun, listExpiredBrokerLeases, recordBrokerReconciliationRun, registerBrokerSession } from './db'
import { revokeLease } from './openbao-proxy'
import { isPrincipalSessionRevoked } from './principal-revocation'
import { logger } from './logger'

let reconcilerTimer: ReturnType<typeof setInterval> | undefined
let lastReconciliationAt: string | null = null
let lastReconciliationResult: { attempted: number; closed: number; failed: number } | null = null
let lastReconciliationError: string | null = null
let reconcilerIntervalMs: number | null = null

/** Register before checking so an offboarding scan cannot miss this session. */
export async function admitBrokerSession(
  sessionId: string, principal: string, leaseId?: string, tenant?: string, authTime?: number, target?: string, leaseExpiresAt?: string,
): Promise<void> {
  if (target || leaseExpiresAt) registerBrokerSession(sessionId, leaseId, principal, tenant, target, leaseExpiresAt)
  else registerBrokerSession(sessionId, leaseId, principal, tenant)
  if (isPrincipalSessionRevoked(principal, authTime)) {
    await closeBrokerSessionAndLease(sessionId)
    throw new Error('Unauthorized: principal revoked during session creation')
  }
}

/** Close the broker session first, then revoke an actual OpenBao lease if one was attached. */
export async function closeBrokerSessionAndLease(sessionId: string): Promise<void> {
  const record = getBrokerSession(sessionId)
  if (!record) throw new Error('Unknown broker session')
  if (record.closed_at) return
  // Each cleanup must be attempted even when the other integration fails.
  // Leave the record open on partial failure so a retry can finish cleanup.
  const results = await Promise.allSettled([
    closeRustGuacSession(sessionId),
    record.lease_id ? revokeLease(record.lease_id) : Promise.resolve(),
  ])
  const failed = results.filter((result) => result.status === 'rejected')
  if (failed.length) throw new Error('Session cleanup incomplete; retry required')
  closeBrokerSession(sessionId)
}

/** Reconcile only leases emitted by this console and already past their TTL. */
export async function reconcileExpiredBrokerLeases(now?: string): Promise<{
  attempted: number
  closed: number
  failed: number
}> {
  const startedAt = new Date().toISOString()
  try {
    const expired = listExpiredBrokerLeases(now)
    let closed = 0
    let failed = 0
    for (const row of expired) {
      try {
        await closeBrokerSessionAndLease(row.session_id)
        closed += 1
      } catch {
        failed += 1
      }
    }
    const result = { attempted: expired.length, closed, failed }
    lastReconciliationAt = new Date().toISOString()
    lastReconciliationResult = result
    lastReconciliationError = null
    recordBrokerReconciliationRun(startedAt, result)
    return result
  } catch (error) {
    lastReconciliationAt = new Date().toISOString()
    lastReconciliationResult = null
    lastReconciliationError = (error as Error).message.slice(0, 200)
    try {
      recordBrokerReconciliationRun(startedAt, { attempted: 0, closed: 0, failed: 0 }, lastReconciliationError)
    } catch {
      /* preserve the original reconciliation error if the audit DB is unavailable */
    }
    throw error
  }
}

export function getBrokerLeaseReconcilerStatus(): {
  enabled: boolean
  interval_ms: number | null
  last_run_at: string | null
  last_result: { attempted: number; closed: number; failed: number } | null
  last_error: string | null
} {
  let persisted = null
  try {
    persisted = getLatestBrokerReconciliationRun()
  } catch {
    /* test doubles or an unavailable local database must not break the probe */
  }
  return {
    enabled: process.env.BROKER_LEASE_RECONCILER_ENABLED === '1',
    interval_ms: reconcilerIntervalMs,
    last_run_at: lastReconciliationAt || persisted?.finished_at || null,
    last_result: lastReconciliationResult || (persisted ? { attempted: persisted.attempted, closed: persisted.closed, failed: persisted.failed } : null),
    last_error: lastReconciliationError || persisted?.error || null,
  }
}

/** Start only when explicitly enabled; one process must own this loop. */
export function startBrokerLeaseReconciler(): void {
  if (reconcilerTimer || process.env.BROKER_LEASE_RECONCILER_ENABLED !== '1') return
  const configured = Number(process.env.BROKER_LEASE_RECONCILER_INTERVAL_MS)
  const intervalMs = Number.isFinite(configured)
    ? Math.min(Math.max(configured, 5_000), 15 * 60_000)
    : 30_000
  reconcilerIntervalMs = intervalMs
  reconcilerTimer = setInterval(() => {
    void reconcileExpiredBrokerLeases().then((result) => {
      if (result.attempted || result.failed) logger.info(result, 'broker lease reconciliation cycle')
    }).catch((error) => {
      logger.warn({ err: String(error) }, 'broker lease reconciliation failed')
    })
  }, intervalMs)
  reconcilerTimer.unref?.()
}

startBrokerLeaseReconciler()
