// W-C3 — Unified audit timeline (console activity + best-effort Warpgate)

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { queryActivityLog } from './activity-log'
import {
  listWarpgateSessions,
  warpgateConfigured,
} from './warpgate-proxy'
import { requireAnyPerm, requireSession } from './session-guard'

export type TimelineEvent = {
  id: string
  timestamp: string
  source: 'console' | 'warpgate' | 'orchestration'
  actor: string
  action: string
  target?: string
  result: 'success' | 'error' | 'info'
  detail?: string
}

export const listUnifiedAuditFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    if (data == null || data === undefined) return { limit: 200, source: 'all' as const }
    const r = z
      .object({
        limit: z.number().int().min(10).max(500).optional(),
        source: z.enum(['all', 'console', 'warpgate']).optional(),
      })
      .safeParse(data)
    if (!r.success) return { limit: 200, source: 'all' as const }
    return {
      limit: r.data.limit ?? 200,
      source: r.data.source ?? ('all' as const),
    }
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['audit:read', 'system:admin', 'persons:read', 'gateways:read'],
      'audit:read',
    )

    const events: TimelineEvent[] = []
    const limit = data.limit

    if (data.source === 'all' || data.source === 'console') {
      const logs = queryActivityLog({ limit })
      for (const e of logs) {
        events.push({
          id: `console-${e.id}`,
          timestamp: e.timestamp,
          source: 'console',
          actor: e.actor,
          action: e.action,
          target: e.target || e.path,
          result: e.result === 'error' ? 'error' : 'success',
          detail: e.errorMessage || `${e.method} ${e.path}`,
        })
      }
    }

    if (
      (data.source === 'all' || data.source === 'warpgate') &&
      warpgateConfigured()
    ) {
      try {
        const sessions = await listWarpgateSessions()
        for (const sess of sessions) {
          events.push({
            id: `wg-sess-${sess.id}`,
            timestamp: sess.started_at || new Date().toISOString(),
            source: 'warpgate',
            actor: sess.username || 'unknown',
            action: 'Sessão ativa',
            target: sess.target,
            result: 'info',
            detail: [sess.protocol, sess.address, sess.id]
              .filter(Boolean)
              .join(' · '),
          })
        }
      } catch {
        // WG version may not expose sessions — skip
      }
    }

    events.sort(
      (a, b) =>
        new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
    )

    return {
      events: events.slice(0, limit),
      sources: {
        console: events.filter((e) => e.source === 'console').length,
        warpgate: events.filter((e) => e.source === 'warpgate').length,
      },
    }
  })
