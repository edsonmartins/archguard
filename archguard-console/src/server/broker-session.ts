import { closeRustGuacSession } from './rustguac-proxy'
import { closeBrokerSession, getBrokerSession } from './db'
import { revokeLease } from './openbao-proxy'

/** Close the broker session first, then revoke an actual OpenBao lease if one was attached. */
export async function closeBrokerSessionAndLease(sessionId: string): Promise<void> {
  const record = getBrokerSession(sessionId)
  await closeRustGuacSession(sessionId)
  if (record?.lease_id && !record.closed_at) await revokeLease(record.lease_id)
  closeBrokerSession(sessionId)
}
