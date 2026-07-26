// Org checkout sessions (ADR-013 OCB-1) — audit trail; secrets never stored here.

import { randomUUID } from 'node:crypto'
import { getDb } from './db'

export type OrgCheckoutStatus =
  | 'pending'
  | 'active'
  | 'denied'
  | 'expired'
  | 'checked_in'

export type OrgCheckout = {
  id: string
  account_id: string
  account_slug: string
  principal: string
  reason: string
  ttl_seconds: number
  status: OrgCheckoutStatus
  approved_by: string | null
  created_at: string
  expires_at: string
  closed_at: string | null
}

type Row = {
  id: string
  account_id: string
  account_slug: string
  principal: string
  reason: string
  ttl_seconds: number
  status: string
  approved_by: string | null
  created_at: string
  expires_at: string
  closed_at: string | null
}

function rowToCheckout(r: Row): OrgCheckout {
  return {
    id: r.id,
    account_id: r.account_id,
    account_slug: r.account_slug,
    principal: r.principal,
    reason: r.reason,
    ttl_seconds: r.ttl_seconds,
    status: r.status as OrgCheckoutStatus,
    approved_by: r.approved_by,
    created_at: r.created_at,
    expires_at: r.expires_at,
    closed_at: r.closed_at,
  }
}

/** Mark overdue active checkouts as expired. */
export function expireDueCheckouts(): number {
  const now = new Date().toISOString()
  const res = getDb()
    .prepare(
      `UPDATE org_checkouts
       SET status = 'expired', closed_at = @now
       WHERE status = 'active' AND expires_at < @now`,
    )
    .run({ now })
  return res.changes
}

export function createCheckout(input: {
  account_id: string
  account_slug: string
  principal: string
  reason: string
  ttl_seconds: number
  status: OrgCheckoutStatus
  approved_by?: string | null
}): OrgCheckout {
  const now = new Date()
  const expires = new Date(now.getTime() + input.ttl_seconds * 1000)
  const row: Row = {
    id: randomUUID(),
    account_id: input.account_id,
    account_slug: input.account_slug,
    principal: input.principal,
    reason: input.reason.trim(),
    ttl_seconds: input.ttl_seconds,
    status: input.status,
    approved_by: input.approved_by ?? null,
    created_at: now.toISOString(),
    expires_at: expires.toISOString(),
    closed_at: null,
  }
  getDb()
    .prepare(
      `INSERT INTO org_checkouts (
        id, account_id, account_slug, principal, reason, ttl_seconds,
        status, approved_by, created_at, expires_at, closed_at
      ) VALUES (
        @id, @account_id, @account_slug, @principal, @reason, @ttl_seconds,
        @status, @approved_by, @created_at, @expires_at, @closed_at
      )`,
    )
    .run(row)
  return rowToCheckout(row)
}

export function getCheckout(id: string): OrgCheckout | null {
  expireDueCheckouts()
  const row = getDb()
    .prepare(`SELECT * FROM org_checkouts WHERE id = ?`)
    .get(id) as Row | undefined
  return row ? rowToCheckout(row) : null
}

export function checkinCheckout(
  id: string,
  principal: string,
): OrgCheckout | null {
  expireDueCheckouts()
  const c = getCheckout(id)
  if (!c) return null
  if (c.status !== 'active' && c.status !== 'pending') return c
  // Owner or will be validated by caller for admin
  const now = new Date().toISOString()
  getDb()
    .prepare(
      `UPDATE org_checkouts
       SET status = 'checked_in', closed_at = ?
       WHERE id = ? AND (principal = ? OR status = 'active' OR status = 'pending')`,
    )
    .run(now, id, principal)
  return getCheckout(id)
}

export function listCheckoutsForAccount(accountId: string, limit = 20): OrgCheckout[] {
  expireDueCheckouts()
  const rows = getDb()
    .prepare(
      `SELECT * FROM org_checkouts
       WHERE account_id = ?
       ORDER BY created_at DESC
       LIMIT ?`,
    )
    .all(accountId, limit) as Row[]
  return rows.map(rowToCheckout)
}

export function listActiveCheckoutsForPrincipal(principal: string): OrgCheckout[] {
  expireDueCheckouts()
  const rows = getDb()
    .prepare(
      `SELECT * FROM org_checkouts
       WHERE principal = ? AND status = 'active'
       ORDER BY created_at DESC`,
    )
    .all(principal) as Row[]
  return rows.map(rowToCheckout)
}

export const TTL_MIN = 300 // 5 min
export const TTL_MAX = 8 * 3600 // 8 h
export const TTL_DEFAULT = 3600 // 1 h
