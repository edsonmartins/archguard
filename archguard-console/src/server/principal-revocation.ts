import { getDb } from './db'

function repository() {
  const db = getDb()
  db.exec(`CREATE TABLE IF NOT EXISTS revoked_principals (
    principal TEXT PRIMARY KEY, revoked_at TEXT NOT NULL
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
