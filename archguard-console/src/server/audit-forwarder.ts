import { claimAuditOutbox, markAuditFailed, markAuditPublished, recoverStaleAuditClaims } from './audit-outbox'
import { logger } from './logger'

let running = false

/**
 * Best-effort forwarder. Disabled unless an explicit HTTPS endpoint is set;
 * the local outbox remains the durable source until publication succeeds.
 */
export async function forwardAuditBatch(): Promise<number> {
  const endpoint = process.env.AUDIT_OUTBOX_FORWARDER_URL?.trim()
  if (!endpoint || running) return 0
  if (!endpoint.startsWith('https://') && process.env.NODE_ENV === 'production') {
    logger.warn({ endpoint }, 'audit forwarder disabled: production endpoint must use HTTPS')
    return 0
  }
  running = true
  let published = 0
  try {
    recoverStaleAuditClaims()
    const rows = claimAuditOutbox(50)
    for (const row of rows) {
      try {
        const response = await fetch(endpoint, {
          method: 'POST',
          headers: {
            'content-type': 'application/json',
            ...(process.env.AUDIT_OUTBOX_FORWARDER_TOKEN
              ? { authorization: `Bearer ${process.env.AUDIT_OUTBOX_FORWARDER_TOKEN}` }
              : {}),
          },
          body: JSON.stringify({
            version: 1,
            type: row.event_type,
            event_id: row.event_id,
            occurred_at: row.occurred_at,
            payload: JSON.parse(row.payload_json),
          }),
          signal: AbortSignal.timeout(5000),
        })
        if (!response.ok) throw new Error(`forwarder HTTP ${response.status}`)
        markAuditPublished(row.event_id)
        published += 1
      } catch (error) {
        markAuditFailed(row.event_id, String(error))
      }
    }
  } finally {
    running = false
  }
  return published
}
