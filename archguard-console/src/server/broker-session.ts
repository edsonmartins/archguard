import { closeRustGuacSession } from './rustguac-proxy'
import { closeBrokerSession, getBrokerSession, registerBrokerSession } from './db'
import { revokeLease } from './openbao-proxy'
import { isPrincipalSessionRevoked } from './principal-revocation'

/** Register before checking so an offboarding scan cannot miss this session. */
export async function admitBrokerSession(
  sessionId: string, principal: string, leaseId?: string, tenant?: string, authTime?: number,
): Promise<void> {
  registerBrokerSession(sessionId, leaseId, principal, tenant)
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
