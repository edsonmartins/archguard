// src/server/db.ts
//
// SQLite database for the activity log and control-plane idempotency. better-sqlite3 is synchronous, has
// no external server dependency and is fast enough for the audit volume
// the console will produce (a few writes per minute, occasional reads).
// Sites may use PostgreSQL (CONSOLE_DATABASE_URL); activity_log stays on SQLite.

import Database from 'better-sqlite3'
import { randomUUID } from 'node:crypto'
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

    CREATE TABLE IF NOT EXISTS audit_outbox (
      event_id      TEXT PRIMARY KEY,
      occurred_at   TEXT NOT NULL,
      event_type    TEXT NOT NULL,
      payload_json  TEXT NOT NULL,
      status        TEXT NOT NULL DEFAULT 'pending',
      attempts      INTEGER NOT NULL DEFAULT 0,
      available_at  TEXT NOT NULL,
      last_error    TEXT,
      published_at  TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_audit_outbox_pending
      ON audit_outbox (status, available_at);

    CREATE TABLE IF NOT EXISTS bff_idempotency (
      scope_key       TEXT PRIMARY KEY,
      body_hash       TEXT NOT NULL,
      status_code     INTEGER,
      response_json   TEXT,
      replay_status_code INTEGER,
      replay_response_json TEXT,
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
      principal   TEXT,
      tenant      TEXT,
      created_at  TEXT NOT NULL,
      closed_at   TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_broker_sessions_open ON broker_sessions (closed_at);
    CREATE TABLE IF NOT EXISTS broker_reconciliation_runs (
      run_id       TEXT PRIMARY KEY,
      started_at   TEXT NOT NULL,
      finished_at  TEXT NOT NULL,
      attempted    INTEGER NOT NULL,
      closed       INTEGER NOT NULL,
      failed       INTEGER NOT NULL,
      error        TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_broker_reconciliation_runs_finished
      ON broker_reconciliation_runs (finished_at DESC);

    CREATE TABLE IF NOT EXISTS access_grants (
      grant_id    TEXT PRIMARY KEY,
      principal   TEXT NOT NULL,
      identity_id TEXT,
      target      TEXT NOT NULL,
      tenant      TEXT,
      role        TEXT,
      created_at  TEXT NOT NULL,
      expires_at  TEXT NOT NULL,
      revoked_at  TEXT,
      source      TEXT NOT NULL DEFAULT 'console'
    );
    CREATE INDEX IF NOT EXISTS idx_access_grants_lookup
      ON access_grants (principal, target, expires_at, revoked_at);

    CREATE TABLE IF NOT EXISTS offboarding_operations (
      operation_id TEXT PRIMARY KEY,
      principal    TEXT NOT NULL,
      status       TEXT NOT NULL,
      started_at   TEXT NOT NULL,
      updated_at   TEXT NOT NULL,
      started_by   TEXT NOT NULL,
      error        TEXT
    );
    CREATE UNIQUE INDEX IF NOT EXISTS idx_offboarding_running
      ON offboarding_operations (principal) WHERE status = 'running';
    CREATE TABLE IF NOT EXISTS offboarding_operation_steps (
      operation_id TEXT NOT NULL,
      sequence     INTEGER NOT NULL,
      component    TEXT NOT NULL,
      ok           INTEGER NOT NULL,
      detail       TEXT,
      recorded_at  TEXT NOT NULL,
      PRIMARY KEY (operation_id, sequence),
      FOREIGN KEY (operation_id) REFERENCES offboarding_operations(operation_id)
    );

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
    CREATE TABLE IF NOT EXISTS connector_inventory_history (
      id                INTEGER PRIMARY KEY AUTOINCREMENT,
      connector_id      TEXT NOT NULL,
      message_id        TEXT NOT NULL,
      status            TEXT NOT NULL,
      agent_version     TEXT NOT NULL,
      observed_at       TEXT NOT NULL,
      inventory_json    TEXT NOT NULL DEFAULT '{}'
    );
    CREATE INDEX IF NOT EXISTS idx_connector_inventory_history_lookup
      ON connector_inventory_history (connector_id, observed_at DESC);

    CREATE TABLE IF NOT EXISTS connector_certificates (
      serial_number TEXT PRIMARY KEY,
      connector_id  TEXT NOT NULL,
      site_slug     TEXT NOT NULL,
      issued_at     TEXT NOT NULL,
      status        TEXT NOT NULL DEFAULT 'active',
      revoked_at    TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_connector_certificates_active
      ON connector_certificates (connector_id, status);

    CREATE TABLE IF NOT EXISTS recording_retention (
      recording_name TEXT PRIMARY KEY,
      legal_hold     INTEGER NOT NULL DEFAULT 0,
      retain_until   TEXT,
      updated_at     TEXT NOT NULL,
      updated_by     TEXT NOT NULL
    );
    CREATE INDEX IF NOT EXISTS idx_recording_retention_until
      ON recording_retention (retain_until);

    CREATE TABLE IF NOT EXISTS connector_upgrade_plans (
      id            TEXT PRIMARY KEY,
      site_slug     TEXT NOT NULL,
      version       TEXT NOT NULL,
      artifact_url  TEXT NOT NULL,
      sha256        TEXT NOT NULL,
      status        TEXT NOT NULL DEFAULT 'pending_approval',
      created_at    TEXT NOT NULL,
      created_by    TEXT NOT NULL,
      decided_at    TEXT,
      decided_by    TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_connector_upgrade_plans_site
      ON connector_upgrade_plans (site_slug, created_at DESC);

    CREATE TABLE IF NOT EXISTS connector_upgrade_rollouts (
      id          TEXT PRIMARY KEY,
      version     TEXT NOT NULL,
      artifact_url TEXT NOT NULL,
      sha256      TEXT NOT NULL,
      status      TEXT NOT NULL DEFAULT 'planned',
      batch_size  INTEGER NOT NULL DEFAULT 1,
      created_at  TEXT NOT NULL,
      created_by  TEXT NOT NULL,
      updated_at  TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS connector_upgrade_rollout_targets (
      id          TEXT PRIMARY KEY,
      rollout_id  TEXT NOT NULL,
      site_slug   TEXT NOT NULL,
      plan_id     TEXT NOT NULL,
      position    INTEGER NOT NULL,
      status      TEXT NOT NULL DEFAULT 'pending',
      error       TEXT,
      started_at  TEXT,
      finished_at TEXT,
      UNIQUE (rollout_id, site_slug),
      FOREIGN KEY (rollout_id) REFERENCES connector_upgrade_rollouts(id)
    );
    CREATE INDEX IF NOT EXISTS idx_connector_upgrade_rollout_targets_status
      ON connector_upgrade_rollout_targets (rollout_id, status, position);
  `)
  for (const statement of [
    'ALTER TABLE activity_log ADD COLUMN principal TEXT',
    'ALTER TABLE activity_log ADD COLUMN tenant_ids TEXT',
    'ALTER TABLE audit_outbox ADD COLUMN claimed_at TEXT',
    'ALTER TABLE broker_sessions ADD COLUMN target TEXT',
    'ALTER TABLE broker_sessions ADD COLUMN lease_expires_at TEXT',
    'ALTER TABLE access_grants ADD COLUMN subject TEXT',
    'ALTER TABLE access_grants ADD COLUMN object TEXT',
    'ALTER TABLE access_grants ADD COLUMN identity_id TEXT',
  ]) {
    try { db.exec(statement) } catch { /* column already exists */ }
  }
  for (const statement of [
    'ALTER TABLE connector_upgrade_plans ADD COLUMN decided_at TEXT',
    'ALTER TABLE connector_upgrade_plans ADD COLUMN decided_by TEXT',
  ]) {
    try { db.exec(statement) } catch { /* column already exists */ }
  }
  // Migrate older DBs that lack multi-connector column
  try {
    const cols = db.prepare(`PRAGMA table_info(sites)`).all() as { name: string }[]
    if (cols.length && !cols.some((c) => c.name === 'connectors_json')) {
      db.exec(`ALTER TABLE sites ADD COLUMN connectors_json TEXT NOT NULL DEFAULT '[]'`)
    }
  } catch {
    /* ignore race / fresh create */
  }
  try {
    const cols = db.prepare('PRAGMA table_info(broker_sessions)').all() as { name: string }[]
    if (!cols.some((c) => c.name === 'principal')) db.exec('ALTER TABLE broker_sessions ADD COLUMN principal TEXT')
    if (!cols.some((c) => c.name === 'tenant')) db.exec('ALTER TABLE broker_sessions ADD COLUMN tenant TEXT')
  } catch { /* existing installations migrate on next startup */ }
  try {
    const cols = db.prepare('PRAGMA table_info(bff_idempotency)').all() as { name: string }[]
    if (!cols.some((c) => c.name === 'replay_status_code')) db.exec('ALTER TABLE bff_idempotency ADD COLUMN replay_status_code INTEGER')
    if (!cols.some((c) => c.name === 'replay_response_json')) db.exec('ALTER TABLE bff_idempotency ADD COLUMN replay_response_json TEXT')
  } catch { /* existing installations migrate on next startup */ }
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

export function registerBrokerSession(sessionId: string, leaseId?: string, principal?: string, tenant?: string, target?: string, leaseExpiresAt?: string): void {
  getDb().prepare(
    `INSERT OR REPLACE INTO broker_sessions (session_id, lease_id, principal, tenant, target, lease_expires_at, created_at, closed_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
  ).run(sessionId, leaseId || null, principal || null, tenant || null, target || null, leaseExpiresAt || null, new Date().toISOString())
}

export function getBrokerSession(sessionId: string): { lease_id: string | null; principal: string | null; tenant: string | null; target: string | null; lease_expires_at: string | null; closed_at: string | null } | undefined {
  return getDb().prepare(
    'SELECT lease_id, principal, tenant, target, lease_expires_at, closed_at FROM broker_sessions WHERE session_id = ?',
  ).get(sessionId) as { lease_id: string | null; principal: string | null; tenant: string | null; target: string | null; lease_expires_at: string | null; closed_at: string | null } | undefined
}

export type BrokerLeaseInventory = {
  session_id: string
  lease_id: string
  principal: string
  tenant: string | null
  target: string | null
  lease_expires_at: string | null
  closed_at: string | null
}

/** Inventory is limited to leases emitted and owned by this console. */
export function listBrokerLeaseInventory(): BrokerLeaseInventory[] {
  return getDb().prepare(
    `SELECT session_id, lease_id, principal, tenant, target, lease_expires_at, closed_at
       FROM broker_sessions WHERE lease_id IS NOT NULL ORDER BY created_at DESC, session_id ASC`,
  ).all() as BrokerLeaseInventory[]
}

export function listExpiredBrokerLeases(now = new Date().toISOString()): BrokerLeaseInventory[] {
  return getDb().prepare(
    `SELECT session_id, lease_id, principal, tenant, target, lease_expires_at, closed_at
       FROM broker_sessions WHERE lease_id IS NOT NULL AND closed_at IS NULL
       AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?`,
  ).all(now) as BrokerLeaseInventory[]
}

export type BrokerReconciliationRun = {
  run_id: string
  started_at: string
  finished_at: string
  attempted: number
  closed: number
  failed: number
  error: string | null
}

export function recordBrokerReconciliationRun(
  startedAt: string,
  result: { attempted: number; closed: number; failed: number },
  error?: string,
): BrokerReconciliationRun {
  const run = {
    run_id: randomUUID(),
    started_at: startedAt,
    finished_at: new Date().toISOString(),
    attempted: result.attempted,
    closed: result.closed,
    failed: result.failed,
    error: error?.slice(0, 500) || null,
  }
  getDb().prepare(`INSERT INTO broker_reconciliation_runs
    (run_id, started_at, finished_at, attempted, closed, failed, error)
    VALUES (?, ?, ?, ?, ?, ?, ?)`).run(
    run.run_id, run.started_at, run.finished_at, run.attempted, run.closed, run.failed, run.error,
  )
  return run
}

export function getLatestBrokerReconciliationRun(): BrokerReconciliationRun | null {
  return (getDb().prepare(`SELECT run_id, started_at, finished_at, attempted, closed, failed, error
    FROM broker_reconciliation_runs ORDER BY finished_at DESC LIMIT 1`).get() as BrokerReconciliationRun | undefined) || null
}

export function listBrokerSessionsForPrincipal(principal: string): string[] {
  return (getDb().prepare(
    'SELECT session_id FROM broker_sessions WHERE principal = ? AND closed_at IS NULL',
  ).all(principal) as { session_id: string }[]).map((row) => row.session_id)
}

export type AccessGrant = {
  grant_id: string
  principal: string
  target: string
  tenant: string | null
  role: string | null
  created_at: string
  expires_at: string
  revoked_at: string | null
  source: string
  identity_id: string | null
  subject: string | null
  object: string | null
}

export function createAccessGrant(input: {
  grant_id: string
  principal: string
  identity_id?: string
  target: string
  tenant?: string
  role?: string
  expires_at: string
  source?: string
  subject?: string
  object?: string
}): AccessGrant {
  const created_at = new Date().toISOString()
  getDb().prepare(
    `INSERT INTO access_grants
      (grant_id, principal, identity_id, target, tenant, role, created_at, expires_at, revoked_at, source, subject, object)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`,
  ).run(input.grant_id, input.principal, input.identity_id || null, input.target, input.tenant || null,
    input.role || null, created_at, input.expires_at, input.source || 'console', input.subject || null, input.object || null)
  return getAccessGrant(input.grant_id)!
}

export function getAccessGrant(grantId: string): AccessGrant | undefined {
  return getDb().prepare(
    'SELECT grant_id, principal, identity_id, target, tenant, role, created_at, expires_at, revoked_at, source, subject, object FROM access_grants WHERE grant_id = ?',
  ).get(grantId) as AccessGrant | undefined
}

export function getLatestAccessGrant(principal: string, target: string): AccessGrant | undefined {
  return getDb().prepare(
    `SELECT grant_id, principal, identity_id, target, tenant, role, created_at, expires_at, revoked_at, source, subject, object
       FROM access_grants WHERE principal = ? AND target = ? ORDER BY created_at DESC LIMIT 1`,
  ).get(principal, target) as AccessGrant | undefined
}

export type AccessGrantPage = {
  items: AccessGrant[]
  total: number
  offset: number
  limit: number
  has_more: boolean
}

export function listAccessGrantsForPrincipal(principal: string, limit = 25, offset = 0): AccessGrantPage {
  const safeLimit = Math.max(1, Math.min(Math.trunc(limit), 100))
  const safeOffset = Math.max(0, Math.trunc(offset))
  const db = getDb()
  const total = (db.prepare('SELECT COUNT(*) AS count FROM access_grants WHERE principal = ?').get(principal) as { count: number }).count
  const items = db.prepare(
    `SELECT grant_id, principal, identity_id, target, tenant, role, created_at, expires_at, revoked_at, source, subject, object
       FROM access_grants WHERE principal = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
  ).all(principal, safeLimit, safeOffset) as AccessGrant[]
  return { items, total, offset: safeOffset, limit: safeLimit, has_more: safeOffset + items.length < total }
}

export function revokeAccessGrant(grantId: string): AccessGrant | undefined {
  getDb().prepare(
    'UPDATE access_grants SET revoked_at = ? WHERE grant_id = ? AND revoked_at IS NULL',
  ).run(new Date().toISOString(), grantId)
  return getAccessGrant(grantId)
}

/**
 * Returns undefined when no console grant exists, otherwise whether at least
 * one non-revoked grant is still valid. This preserves overlapping grants.
 */
export function hasActiveAccessGrant(principal: string, target: string, now?: number): boolean | undefined
export function hasActiveAccessGrant(principal: string, target: string, identityId?: string, now?: number): boolean | undefined
export function hasActiveAccessGrant(principal: string, target: string, identityIdOrNow?: string | number, now = Date.now()): boolean | undefined {
  const identityId = typeof identityIdOrNow === 'string' ? identityIdOrNow : undefined
  const effectiveNow = typeof identityIdOrNow === 'number' ? identityIdOrNow : now
  const rows = getDb().prepare(
    `SELECT expires_at, revoked_at, identity_id FROM access_grants
       WHERE principal = ? AND target = ?`,
  ).all(principal, target) as Array<{ expires_at: string; revoked_at: string | null; identity_id: string | null }>
  if (rows.length === 0) return undefined
  const scopedRows = identityId && rows.some((row) => row.identity_id)
    ? rows.filter((row) => row.identity_id === identityId)
    : rows
  if (scopedRows.length === 0) return false
  return scopedRows.some((row) => !row.revoked_at && new Date(row.expires_at).getTime() > effectiveNow)
}

export function revokeAccessGrantsForPrincipal(principal: string): number {
  return getDb().prepare(
    'UPDATE access_grants SET revoked_at = ? WHERE principal = ? AND revoked_at IS NULL',
  ).run(new Date().toISOString(), principal).changes
}

export type LegacyAccessGrantStatus = {
  total: number
  principals: number
  oldest_created_at: string | null
}

export function getLegacyAccessGrantStatus(): LegacyAccessGrantStatus {
  const row = getDb().prepare(`SELECT COUNT(*) AS total,
    COUNT(DISTINCT principal) AS principals,
    MIN(created_at) AS oldest_created_at
    FROM access_grants WHERE identity_id IS NULL`).get() as { total: number; principals: number; oldest_created_at: string | null }
  return row
}

export type LegacyAccessGrantMigration = {
  principal: string
  identity_id: string
  affected: number
}

/**
 * Attach an explicitly verified canonical identity to legacy grants.
 * This deliberately never overwrites an existing identity or infers one.
 */
export function migrateLegacyAccessGrants(principal: string, identityId: string): LegacyAccessGrantMigration {
  const normalizedPrincipal = principal.trim()
  const normalizedIdentityId = identityId.trim()
  if (!normalizedPrincipal || !normalizedIdentityId) {
    throw new Error('principal e identity_id são obrigatórios')
  }
  const db = getDb()
  const affected = db.prepare(
    'UPDATE access_grants SET identity_id = ? WHERE principal = ? AND identity_id IS NULL',
  ).run(normalizedIdentityId, normalizedPrincipal).changes
  return { principal: normalizedPrincipal, identity_id: normalizedIdentityId, affected }
}

export function countLegacyAccessGrantsForPrincipal(principal: string): number {
  return (getDb().prepare(
    'SELECT COUNT(*) AS count FROM access_grants WHERE principal = ? AND identity_id IS NULL',
  ).get(principal.trim()) as { count: number }).count
}

export function listLegacyAccessGrantsForPrincipal(principal: string): AccessGrant[] {
  return getDb().prepare(
    `SELECT grant_id, principal, identity_id, target, tenant, role, created_at, expires_at, revoked_at, source, subject, object
       FROM access_grants WHERE principal = ? AND identity_id IS NULL ORDER BY created_at ASC`,
  ).all(principal.trim()) as AccessGrant[]
}

export function beginOffboardingOperation(principal: string, actor: string, staleAfterMs = 15 * 60_000): string {
  const db = getDb()
  const now = new Date().toISOString()
  const staleBefore = new Date(Date.now() - Math.max(60_000, staleAfterMs)).toISOString()
  db.prepare(`UPDATE offboarding_operations SET status = 'partial', updated_at = ?, error = 'operation stale; retry scheduled'
    WHERE principal = ? AND status = 'running' AND updated_at <= ?`).run(now, principal, staleBefore)
  const resumable = db.prepare(`SELECT operation_id FROM offboarding_operations
    WHERE principal = ? AND status = 'partial' ORDER BY updated_at DESC LIMIT 1`).get(principal) as { operation_id: string } | undefined
  if (resumable) {
    db.prepare(`UPDATE offboarding_operations SET status = 'running', updated_at = ?, started_by = ?, error = NULL
      WHERE operation_id = ?`).run(now, actor, resumable.operation_id)
    return resumable.operation_id
  }
  const operationId = randomUUID()
  try {
    db.prepare(`INSERT INTO offboarding_operations
      (operation_id, principal, status, started_at, updated_at, started_by, error)
      VALUES (?, ?, 'running', ?, ?, ?, NULL)`).run(operationId, principal, now, now, actor)
    return operationId
  } catch (error) {
    if (/UNIQUE/i.test(String(error))) throw new Error('Offboarding já está em execução para este principal')
    throw error
  }
}

export function finishOffboardingOperation(operationId: string, status: 'completed' | 'partial', error?: string): void {
  getDb().prepare(`UPDATE offboarding_operations SET status = ?, updated_at = ?, error = ? WHERE operation_id = ?`)
    .run(status, new Date().toISOString(), error?.slice(0, 500) || null, operationId)
}

export function recordOffboardingStep(operationId: string, sequence: number, step: { component: string; ok: boolean; detail?: string }): void {
  getDb().prepare(`INSERT OR REPLACE INTO offboarding_operation_steps
    (operation_id, sequence, component, ok, detail, recorded_at) VALUES (?, ?, ?, ?, ?, ?)`)
    .run(operationId, sequence, step.component, step.ok ? 1 : 0, step.detail?.slice(0, 500) || null, new Date().toISOString())
}

export function listOffboardingSteps(operationId: string): OffboardingOperation['steps'] {
  return (getDb().prepare(
    `SELECT sequence, component, ok, detail, recorded_at FROM offboarding_operation_steps
       WHERE operation_id = ? ORDER BY sequence ASC`,
  ).all(operationId) as Array<{ sequence: number; component: string; ok: number; detail: string | null; recorded_at: string }>)
    .map((step) => ({ ...step, ok: step.ok === 1 }))
}

export type OffboardingOperation = {
  operation_id: string
  principal: string
  status: 'running' | 'completed' | 'partial'
  started_at: string
  updated_at: string
  started_by: string
  error: string | null
  steps: Array<{ sequence: number; component: string; ok: boolean; detail: string | null; recorded_at: string }>
}

export function listOffboardingOperationsForPrincipal(principal: string, limit = 10): OffboardingOperation[] {
  const operations = getDb().prepare(
    `SELECT operation_id, principal, status, started_at, updated_at, started_by, error
       FROM offboarding_operations WHERE principal = ? ORDER BY started_at DESC LIMIT ?`,
  ).all(principal, Math.max(1, Math.min(limit, 50))) as Array<Omit<OffboardingOperation, 'steps'>>
  return operations.map((operation) => ({
    ...operation,
    status: operation.status as OffboardingOperation['status'],
    steps: listOffboardingSteps(operation.operation_id),
  }))
}

/** Historical ownership index used for recording access after session close. */
export function listBrokerRecordingSessionsForPrincipal(principal: string): string[] {
  return (getDb().prepare(
    `SELECT session_id FROM broker_sessions
       WHERE principal = ? AND session_id IS NOT NULL`,
  ).all(principal) as { session_id: string }[]).map((row) => row.session_id)
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
