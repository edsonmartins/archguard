// src/server/db.ts
//
// SQLite database for the activity log and control-plane idempotency. better-sqlite3 is synchronous, has
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

    CREATE TABLE IF NOT EXISTS bff_idempotency (
      scope_key       TEXT PRIMARY KEY,
      body_hash       TEXT NOT NULL,
      status_code     INTEGER,
      response_json   TEXT,
      created_at      TEXT NOT NULL,
      completed_at    TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_bff_idempotency_created ON bff_idempotency (created_at);

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

    -- Org Credential Broker (ADR-013) — metadata only; secrets in OpenBao
    CREATE TABLE IF NOT EXISTS org_accounts (
      id                      TEXT PRIMARY KEY,
      slug                    TEXT NOT NULL UNIQUE,
      name                    TEXT NOT NULL,
      category                TEXT NOT NULL DEFAULT 'other',
      product                 TEXT NOT NULL DEFAULT '',
      url                     TEXT NOT NULL DEFAULT '',
      login_hint              TEXT NOT NULL DEFAULT '',
      auth_kind               TEXT NOT NULL DEFAULT 'password',
      federation_status       TEXT NOT NULL DEFAULT 'password_only',
      oidc_client_id          TEXT NOT NULL DEFAULT '',
      secret_ref              TEXT NOT NULL DEFAULT '',
      criticality             TEXT NOT NULL DEFAULT 'P2',
      owner_group             TEXT NOT NULL DEFAULT '',
      requires_dual_control   INTEGER NOT NULL DEFAULT 0,
      notes                   TEXT NOT NULL DEFAULT '',
      runbook_url             TEXT NOT NULL DEFAULT '',
      rotated_at              TEXT,
      updated_at              TEXT NOT NULL,
      updated_by              TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_org_accounts_category ON org_accounts (category);
    CREATE INDEX IF NOT EXISTS idx_org_accounts_criticality ON org_accounts (criticality);

    CREATE TABLE IF NOT EXISTS org_checkouts (
      id              TEXT PRIMARY KEY,
      account_id      TEXT NOT NULL,
      account_slug    TEXT NOT NULL,
      principal       TEXT NOT NULL,
      reason          TEXT NOT NULL,
      ttl_seconds     INTEGER NOT NULL,
      status          TEXT NOT NULL,
      approved_by     TEXT,
      created_at      TEXT NOT NULL,
      expires_at      TEXT NOT NULL,
      closed_at       TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_org_checkouts_account ON org_checkouts (account_id);
    CREATE INDEX IF NOT EXISTS idx_org_checkouts_principal ON org_checkouts (principal);
    CREATE INDEX IF NOT EXISTS idx_org_checkouts_status ON org_checkouts (status);

    -- Manager-only ops settings (key/value; never secrets of customers)
    CREATE TABLE IF NOT EXISTS manager_settings (
      key         TEXT PRIMARY KEY,
      value       TEXT NOT NULL DEFAULT '',
      updated_at  TEXT NOT NULL,
      updated_by  TEXT
    );

    CREATE TABLE IF NOT EXISTS broker_sessions (
      session_id  TEXT PRIMARY KEY,
      lease_id    TEXT,
      created_at  TEXT NOT NULL,
      closed_at   TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_broker_sessions_open ON broker_sessions (closed_at);

    -- Single-use connector enrollment metadata; token material is never stored.
    CREATE TABLE IF NOT EXISTS connector_enrollments (
      id            TEXT PRIMARY KEY,
      token_hash    TEXT NOT NULL UNIQUE,
      site_slug     TEXT NOT NULL,
      connector_id  TEXT NOT NULL,
      expires_at    TEXT NOT NULL,
      created_at    TEXT NOT NULL,
      created_by    TEXT NOT NULL,
      used_at       TEXT,
      revoked_at    TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_connector_enrollments_lookup
      ON connector_enrollments (site_slug, connector_id, expires_at);

    CREATE TABLE IF NOT EXISTS connector_heartbeats (
      connector_id       TEXT PRIMARY KEY,
      message_id         TEXT NOT NULL,
      status              TEXT NOT NULL,
      agent_version       TEXT NOT NULL,
      capabilities_json   TEXT NOT NULL DEFAULT '[]',
      last_seen_at        TEXT NOT NULL,
      payload_json        TEXT NOT NULL DEFAULT '{}'
    );
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
  // org_accounts column migrations (OCB-3/4)
  try {
    const cols = db.prepare(`PRAGMA table_info(org_accounts)`).all() as {
      name: string
    }[]
    if (cols.length && !cols.some((c) => c.name === 'rotated_at')) {
      db.exec(`ALTER TABLE org_accounts ADD COLUMN rotated_at TEXT`)
    }
    if (cols.length && !cols.some((c) => c.name === 'federation_status')) {
      db.exec(
        `ALTER TABLE org_accounts ADD COLUMN federation_status TEXT NOT NULL DEFAULT 'password_only'`,
      )
    }
    if (cols.length && !cols.some((c) => c.name === 'oidc_client_id')) {
      db.exec(
        `ALTER TABLE org_accounts ADD COLUMN oidc_client_id TEXT NOT NULL DEFAULT ''`,
      )
    }
  } catch {
    /* ignore */
  }
}

export function getDb(): Database.Database {
  if (!_db) _db = open()
  return _db
}

export function registerBrokerSession(sessionId: string, leaseId?: string): void {
  getDb().prepare(
    `INSERT OR REPLACE INTO broker_sessions (session_id, lease_id, created_at, closed_at)
     VALUES (?, ?, ?, NULL)`,
  ).run(sessionId, leaseId || null, new Date().toISOString())
}

export function getBrokerSession(sessionId: string): { lease_id: string | null; closed_at: string | null } | undefined {
  return getDb().prepare(
    'SELECT lease_id, closed_at FROM broker_sessions WHERE session_id = ?',
  ).get(sessionId) as { lease_id: string | null; closed_at: string | null } | undefined
}

export function closeBrokerSession(sessionId: string): void {
  getDb().prepare(
    'UPDATE broker_sessions SET closed_at = ? WHERE session_id = ? AND closed_at IS NULL',
  ).run(new Date().toISOString(), sessionId)
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
