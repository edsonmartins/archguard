import { closeRustGuacSession } from './rustguac-proxy'
import { closeBrokerSession, getBrokerSession } from './db'
import { revokeLease } from './openbao-proxy'

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
