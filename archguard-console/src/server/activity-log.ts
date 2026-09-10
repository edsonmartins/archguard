// src/server/activity-log.ts
//
// Persisted activity log for tracking mutations made through the console.
// archguard v1.9 has no public audit API, so we record every mutation that
// crosses our proxy. The store survives restarts (SQLite, see db.ts).

import { z } from 'zod'
import { getDb } from './db'
import type { ActivityLogEntry } from '@/lib/api/types/archguard'
import { getSessionOrNull, sessionActor } from './session-guard'
import { logger } from './logger'
import { forwardAuditBatch } from './audit-forwarder'
import { deriveTenants, stripGroupDomain } from '@/lib/auth/roles'

export function getActor(): string {
  const s = getSessionOrNull()
  if (!s) return 'unknown'
  return sessionActor(s)
}

/**
 * Record a mutation in the activity log. Called by archguard-proxy after
 * each write operation, success or failure.
 */
export function recordActivity(
  method: string,
  path: string,
  actor: string,
  result: 'success' | 'error',
  errorMessage?: string,
  body?: unknown,
): void {
  const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const session = getSessionOrNull()
  const principal = session?.user?.name || undefined
  const tenantIds = session
    ? [...new Set(deriveTenants(session.groups).map(stripGroupDomain))]
    : []
  const entry: ActivityLogEntry = {
    id,
    timestamp: new Date().toISOString(),
    actor,
    action: deriveAction(method, path),
    method,
    path,
    // POST /v1/<resource> creates carry the resource name in the body's
    // `attrs.name`, not in the URL — fall back to that so the audit row
    // is searchable by the new entity's identifier.
    target: deriveTarget(path) ?? deriveTargetFromBody(body),
    principal,
    tenantIds,
    result,
    errorMessage,
  }

  try {
    const db = getDb()
    const write = db.transaction(() => {
      db.prepare(
        `INSERT INTO activity_log
           (id, timestamp, actor, action, method, path, target, principal, tenant_ids, result, error_message)
         VALUES
           (@id, @timestamp, @actor, @action, @method, @path, @target, @principal, @tenantIds, @result, @errorMessage)`,
      ).run({ ...entry, tenantIds: JSON.stringify(tenantIds) })
      // Outbox payload is deliberately limited to redacted audit metadata.
      db.prepare(
        `INSERT INTO audit_outbox
           (event_id, occurred_at, event_type, payload_json, available_at)
         VALUES (?, ?, ?, ?, ?)`,
      ).run(
        entry.id,
        entry.timestamp,
        'archgate.activity.v1',
        JSON.stringify({
          event_id: entry.id,
          occurred_at: entry.timestamp,
          actor: entry.actor,
          action: entry.action,
          method: entry.method,
          path: entry.path,
          target: entry.target,
          principal: entry.principal,
          tenant_ids: tenantIds,
          result: entry.result,
          error: entry.errorMessage,
        }),
        entry.timestamp,
      )
    })
    write()
    void forwardAuditBatch()
  } catch (err) {
    // Never fail the primary mutation because audit write failed (e.g. empty/corrupt sqlite).
    logger.warn({ err: String(err), action: entry.action }, 'activity_log insert failed')
  }
}

function deriveAction(method: string, path: string): string {
  const segments = path.split('/').filter(Boolean)
  const resource = segments[1] ?? ''

  // ArchGate control plane
  if (path.startsWith('/archgate/sites')) {
    if (path.includes('/import')) return 'Importar site YAML'
    if (method === 'DELETE') return 'Excluir site'
    if (method === 'PUT' || method === 'POST') return 'Salvar site'
  }
  if (path.startsWith('/archgate/org-accounts')) {
    if (path.includes('/approve')) return 'Aprovar checkout org'
    if (path.includes('/deny')) return 'Negar checkout org'
    if (path.includes('/checkout')) return 'Checkout conta org'
    if (path.includes('/checkin')) return 'Check-in conta org'
    if (method === 'DELETE') return 'Excluir conta org'
    if (method === 'PUT' || method === 'POST') return 'Salvar conta org'
  }
  if (path.startsWith('/archgate/connector')) return 'Checklist connector'
  if (path.includes('/targets/') && path.includes('/secret'))
    return 'Secret de target (OpenBao)'
  if (path.includes('/revoke')) return 'Revogar acesso'
  if (path.includes('/provision')) return 'Provisionar acesso'
  if (path.includes('/grant')) return 'Grant target'
  if (path.includes('onboarding/wizard')) return 'Wizard novo cliente'
  if (path.includes('mentors-axis')) return 'Sync Mentors Axis'

  if (method === 'POST' && !path.includes('/_')) return `Criar ${resource}`
  if (method === 'DELETE' && !path.includes('/_')) return `Excluir ${resource}`
  if (method === 'PUT' || method === 'PATCH') return `Atualizar ${resource}`
  if (path.includes('/_attr/member') && method === 'POST') return 'Adicionar membro'
  if (path.includes('/_attr/member') && method === 'DELETE') return 'Remover membro'
  if (path.includes('/_credential')) return 'Credencial'
  if (path.includes('/_scopemap')) return 'Scope Map'
  if (path.includes('/_claimmap')) return 'Claim Map'
  if (path.includes('/_api_token') && method === 'POST') return 'Gerar token'
  if (path.includes('/_api_token') && method === 'DELETE') return 'Revogar token'
  if (path.includes('/_revive')) return 'Restaurar da lixeira'
  return `${method} ${resource}`
}

function deriveTarget(path: string): string | undefined {
  const segments = path.split('/').filter(Boolean)
  if (segments.length >= 3) {
    const id = segments[2]
    if (id && !id.startsWith('_')) return decodeURIComponent(id)
  }
  return undefined
}

function deriveTargetFromBody(body: unknown): string | undefined {
  if (!body || typeof body !== 'object') return undefined
  const attrs = (body as { attrs?: { name?: string[] } }).attrs
  return attrs?.name?.[0]
}

// ── Server Functions ────────────────────────────

const queryFiltersSchema = z.object({
  limit: z.number().int().min(1).max(1000).optional(),
  offset: z.number().int().min(0).optional(),
  actor: z.string().max(256).optional(),
  result: z.enum(['success', 'error']).optional(),
  since: z.string().datetime().optional(),
  until: z.string().datetime().optional(),
})

export type ActivityLogFilters = z.infer<typeof queryFiltersSchema>

export function queryActivityLog(filters: ActivityLogFilters = {}): ActivityLogEntry[] {
  const conditions: string[] = []
  const params: Record<string, unknown> = {}
  if (filters.actor) {
    conditions.push('actor = @actor')
    params.actor = filters.actor
  }
  if (filters.result) {
    conditions.push('result = @result')
    params.result = filters.result
  }
  if (filters.since) {
    conditions.push('timestamp >= @since')
    params.since = filters.since
  }
  if (filters.until) {
    conditions.push('timestamp <= @until')
    params.until = filters.until
  }
  const where = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
  const limit = filters.limit ?? 200
  const offset = filters.offset ?? 0

  const rows = getDb()
    .prepare(
      `SELECT id, timestamp, actor, action, method, path, target, principal, tenant_ids,
              result,
              error_message AS errorMessage
       FROM activity_log
       ${where}
       ORDER BY timestamp DESC, rowid DESC
       LIMIT ${limit} OFFSET ${offset}`,
    )
    .all(params) as ActivityLogEntry[]

  // SQLite returns null for missing optional columns; normalize to undefined
  // so the response shape matches the in-memory format used by tests.
  return rows.map((r) => ({
    ...r,
    target: r.target ?? undefined,
    principal: r.principal ?? undefined,
    tenantIds: parseTenantIds((r as ActivityLogEntry & { tenant_ids?: string }).tenant_ids),
    errorMessage: r.errorMessage ?? undefined,
  }))
}

function parseTenantIds(value?: string): string[] | undefined {
  if (!value) return undefined
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) && parsed.every((item) => typeof item === 'string') ? parsed : undefined
  } catch { return undefined }
}
