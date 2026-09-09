import { getDb } from './db'

function repository() {
  const db = getDb()
  db.exec(`CREATE TABLE IF NOT EXISTS revoked_principals (
    principal TEXT PRIMARY KEY, revoked_at TEXT NOT NULL
  )`)
  db.exec(`CREATE TABLE IF NOT EXISTS principal_reactivations (
    principal TEXT PRIMARY KEY, authenticated_after INTEGER NOT NULL
  )`)
  return db
}

/** Durable local deny: reactivation requires an explicit administrative flow. */
export function revokePrincipal(principal: string): void {
  if (!principal.trim()) throw new Error('Principal required')
  repository().prepare('INSERT OR REPLACE INTO revoked_principals VALUES (?, ?)')
    .run(principal.trim(), new Date().toISOString())
}

export function isPrincipalRevoked(principal?: string): boolean {
  return Boolean(principal && repository().prepare(
    'SELECT 1 FROM revoked_principals WHERE principal = ?',
  ).get(principal.trim()))
}

export function reactivatePrincipal(principal: string): void {
  if (!principal.trim()) throw new Error('Principal required')
  const db = repository()
  db.transaction(() => {
    db.prepare('INSERT OR REPLACE INTO principal_reactivations VALUES (?, ?)')
      .run(principal.trim(), Math.floor(Date.now() / 1000))
    db.prepare('DELETE FROM revoked_principals WHERE principal = ?').run(principal.trim())
  })()
}

/** authTime must come from verified IdP claims, never a client assertion. */
export function isPrincipalSessionRevoked(principal?: string, authTime?: number): boolean {
  if (isPrincipalRevoked(principal)) return true
  if (!principal) return false
  const row = repository().prepare(
    'SELECT authenticated_after FROM principal_reactivations WHERE principal = ?',
  ).get(principal.trim()) as { authenticated_after: number } | undefined
  return Boolean(row && (!Number.isFinite(authTime) || authTime! <= row.authenticated_after))
}
