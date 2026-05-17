// src/server/db.ts
//
// SQLite database for the activity log. better-sqlite3 is synchronous, has
// no external server dependency and is fast enough for the audit volume
// the console will produce (a few writes per minute, occasional reads).

import Database from 'better-sqlite3'
import { mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

function dbPath(): string {
  return resolve(process.env.ARCHGUARD_DB_PATH || './data/archguard.sqlite')
}

let _db: Database.Database | null = null

function open(): Database.Database {
  const path = dbPath()
  mkdirSync(dirname(path), { recursive: true })
  const db = new Database(path)
  db.pragma('journal_mode = WAL')
  db.pragma('synchronous = NORMAL')
  db.pragma('foreign_keys = ON')
  migrate(db)
  return db
}

function migrate(db: Database.Database): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS activity_log (
      id            TEXT PRIMARY KEY,
      timestamp     TEXT NOT NULL,
      actor         TEXT NOT NULL,
      action        TEXT NOT NULL,
      method        TEXT NOT NULL,
      path          TEXT NOT NULL,
      target        TEXT,
      result        TEXT NOT NULL,
      error_message TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_activity_timestamp ON activity_log (timestamp DESC);
    CREATE INDEX IF NOT EXISTS idx_activity_actor ON activity_log (actor);
  `)
}

export function getDb(): Database.Database {
  if (!_db) _db = open()
  return _db
}

/** For tests: close and re-open against a fresh path. */
export function _resetDbForTests(path?: string): void {
  if (_db) _db.close()
  _db = null
  if (path) process.env.ARCHGUARD_DB_PATH = path
}

export function pingDb(): boolean {
  try {
    const row = getDb().prepare('SELECT 1 as ok').get() as { ok: number }
    return row?.ok === 1
  } catch {
    return false
  }
}
