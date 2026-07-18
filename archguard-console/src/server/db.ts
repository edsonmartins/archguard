// src/server/db.ts
//
// SQLite database for the activity log. better-sqlite3 is synchronous, has
// no external server dependency and is fast enough for the audit volume
// the console will produce (a few writes per minute, occasional reads).
// Sites may use PostgreSQL (CONSOLE_DATABASE_URL); activity_log stays on SQLite.

import Database from 'better-sqlite3'
import { mkdirSync, statSync, unlinkSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { logger } from './logger'

function dbPath(): string {
  return resolve(process.env.ARCHGUARD_DB_PATH || './data/archguard.sqlite')
}

let _db: Database.Database | null = null

/**
 * Remove zero-byte or unreadable SQLite files so migrate() can recreate schema.
 * Seen in lab when volume had an empty archguard.sqlite and activity_log was missing.
 */
function scrubCorruptDbFile(path: string): void {
  try {
    const st = statSync(path)
    if (st.size === 0) {
      unlinkSync(path)
      logger.warn({ path }, 'removed empty sqlite file; will recreate')
    }
  } catch {
    // not found — fine
  }
}

function open(): Database.Database {
  const path = dbPath()
  mkdirSync(dirname(path), { recursive: true })
  scrubCorruptDbFile(path)
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

    -- ArchGate site / client inventory (connectivity + targets metadata)
    CREATE TABLE IF NOT EXISTS sites (
      slug            TEXT PRIMARY KEY,
      cliente         TEXT NOT NULL,
      tenant_group    TEXT NOT NULL,
      ambiente        TEXT NOT NULL DEFAULT 'producao',
      tipo            TEXT NOT NULL DEFAULT 'a_confirmar',
      stack           TEXT NOT NULL DEFAULT 'a_confirmar',
      connector_id    TEXT,
      subnets_json    TEXT NOT NULL DEFAULT '[]',
      stack_meta_json TEXT NOT NULL DEFAULT '{}',
      connectors_json TEXT NOT NULL DEFAULT '[]',
      targets_json    TEXT NOT NULL DEFAULT '[]',
      warpgate_roles_json TEXT NOT NULL DEFAULT '[]',
      notas           TEXT,
      inventariado    INTEGER NOT NULL DEFAULT 0,
      connector_deployed INTEGER NOT NULL DEFAULT 0,
      smoke_operador  INTEGER NOT NULL DEFAULT 0,
      updated_at      TEXT NOT NULL,
      updated_by      TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_sites_stack ON sites (stack);
    CREATE INDEX IF NOT EXISTS idx_sites_tenant ON sites (tenant_group);
  `)
  // Migrate older DBs that lack multi-connector column
  try {
    const cols = db.prepare(`PRAGMA table_info(sites)`).all() as { name: string }[]
    if (cols.length && !cols.some((c) => c.name === 'connectors_json')) {
      db.exec(
        `ALTER TABLE sites ADD COLUMN connectors_json TEXT NOT NULL DEFAULT '[]'`,
      )
    }
  } catch {
    /* ignore race / fresh create */
  }
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
