import { createHash, randomBytes, randomUUID, timingSafeEqual } from 'node:crypto'
import { getDb } from './db'

function digest(token: string): Buffer {
  return createHash('sha256').update(token, 'utf8').digest()
}

export type ConnectorEnrollment = {
  id: string
  site_slug: string
  connector_id: string
  expires_at: string
  created_at: string
}

/** Issue a short-lived, single-use enrollment token. The clear token is returned once. */
export function issueConnectorEnrollment(input: {
  site_slug: string
  connector_id: string
  created_by: string
  ttl_seconds?: number
}): { token: string; enrollment: ConnectorEnrollment } {
  const ttl = Math.min(Math.max(input.ttl_seconds || 900, 60), 3600)
  const token = randomBytes(32).toString('base64url')
  const now = new Date()
  const enrollment = {
    id: randomUUID(),
    site_slug: input.site_slug,
    connector_id: input.connector_id,
    expires_at: new Date(now.getTime() + ttl * 1000).toISOString(),
    created_at: now.toISOString(),
    created_by: input.created_by,
  }
  getDb()
    .prepare(
      `INSERT INTO connector_enrollments
       (id, token_hash, site_slug, connector_id, expires_at, created_at, created_by)
       VALUES (?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      enrollment.id,
      digest(token),
      enrollment.site_slug,
      enrollment.connector_id,
      enrollment.expires_at,
      enrollment.created_at,
      enrollment.created_by,
    )
  return { token, enrollment }
}

/** Atomically consume a token; expired, revoked and replayed tokens fail closed. */
export function consumeConnectorEnrollment(token: string): ConnectorEnrollment | null {
  if (!token || token.length > 256) return null
  const row = getDb()
    .prepare(
      `SELECT id, token_hash, site_slug, connector_id, expires_at, created_at
       FROM connector_enrollments
       WHERE used_at IS NULL AND revoked_at IS NULL AND expires_at > ?`,
    )
    .all(new Date().toISOString()) as Array<ConnectorEnrollment & { token_hash: Buffer }>
  const hash = digest(token)
  const match = row.find((candidate) => {
    const stored = Buffer.from(candidate.token_hash)
    return stored.length === hash.length && timingSafeEqual(stored, hash)
  })
  if (!match) return null
  const changed = getDb()
    .prepare(
      `UPDATE connector_enrollments SET used_at = ?
       WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`,
    )
    .run(new Date().toISOString(), match.id)
  if (changed.changes !== 1) return null
  return {
    id: match.id,
    site_slug: match.site_slug,
    connector_id: match.connector_id,
    expires_at: match.expires_at,
    created_at: match.created_at,
  }
}
