import { getDb } from './db'

export type AuditOutboxRow = {
  event_id: string
  occurred_at: string
  event_type: string
  payload_json: string
  status: 'pending' | 'publishing' | 'published' | 'failed'
  attempts: number
  available_at: string
  last_error: string | null
  published_at: string | null
  claimed_at: string | null
}

export type AuditOutboxStatus = {
  pending: number
  publishing: number
  failed: number
  published: number
  oldest_pending_at: string | null
  last_published_at: string | null
}

export function getAuditOutboxStatus(): AuditOutboxStatus {
  const db = getDb()
  const counts = db.prepare(
    `SELECT status, COUNT(*) AS count FROM audit_outbox GROUP BY status`,
  ).all() as Array<{ status: string; count: number }>
  const byStatus = new Map(counts.map((row) => [row.status, row.count]))
  const oldest = db.prepare(
    `SELECT occurred_at FROM audit_outbox WHERE status IN ('pending', 'failed', 'publishing') ORDER BY occurred_at ASC LIMIT 1`,
  ).get() as { occurred_at?: string } | undefined
  const latest = db.prepare(
    `SELECT published_at FROM audit_outbox WHERE status = 'published' ORDER BY published_at DESC LIMIT 1`,
  ).get() as { published_at?: string } | undefined
  return {
    pending: byStatus.get('pending') || 0,
    publishing: byStatus.get('publishing') || 0,
    failed: byStatus.get('failed') || 0,
    published: byStatus.get('published') || 0,
    oldest_pending_at: oldest?.occurred_at || null,
    last_published_at: latest?.published_at || null,
  }
}

/** Requeue claims left behind by a crashed forwarder. */
export function recoverStaleAuditClaims(maxAgeMs = 60_000): number {
  const cutoff = new Date(Date.now() - Math.max(1_000, maxAgeMs)).toISOString()
  return getDb().prepare(
    `UPDATE audit_outbox
        SET status = 'failed', available_at = ?, claimed_at = NULL,
            last_error = 'forwarder claim expired; retry scheduled'
      WHERE status = 'publishing' AND (claimed_at IS NULL OR claimed_at <= ?)`,
  ).run(new Date().toISOString(), cutoff).changes
}

export function claimAuditOutbox(limit = 50): AuditOutboxRow[] {
  const db = getDb()
  const now = new Date().toISOString()
  const rows = db.prepare(
    `SELECT * FROM audit_outbox
      WHERE status IN ('pending', 'failed') AND available_at <= ?
      ORDER BY occurred_at ASC LIMIT ?`,
  ).all(now, Math.max(1, Math.min(limit, 500))) as AuditOutboxRow[]
  const mark = db.prepare(
    `UPDATE audit_outbox SET status = 'publishing', attempts = attempts + 1, claimed_at = ?
      WHERE event_id = ? AND status IN ('pending', 'failed')`,
  )
  const claimedAt = new Date().toISOString()
  return db.transaction(() => rows.filter((row) => mark.run(claimedAt, row.event_id).changes === 1))()
}

export function markAuditPublished(eventId: string): void {
  getDb().prepare(
    `UPDATE audit_outbox SET status = 'published', published_at = ?, claimed_at = NULL, last_error = NULL
      WHERE event_id = ?`,
  ).run(new Date().toISOString(), eventId)
}

export function markAuditFailed(eventId: string, error: string): void {
  const safeError = error.slice(0, 500)
  getDb().prepare(
    `UPDATE audit_outbox SET status = 'failed', last_error = ?, available_at = ?, claimed_at = NULL
      WHERE event_id = ?`,
  ).run(safeError, new Date(Date.now() + 30_000).toISOString(), eventId)
}
