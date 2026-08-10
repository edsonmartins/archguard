// GET /api/org/v1/connectors/inventory — latest bounded connector inventory.

import { createFileRoute } from '@tanstack/react-router'
import { requireAnyPerm, requireSession } from '@/server/session-guard'
import { getDb } from '@/server/db'

export const Route = createFileRoute('/api/org/v1/connectors/inventory')({
  server: {
    handlers: {
      GET: async () => {
        try {
          const session = requireSession()
          requireAnyPerm(session, ['sites:read', 'sites:update'], 'sites:read')
          const rows = getDb().prepare(
            `SELECT connector_id, status, agent_version, capabilities_json,
                    last_seen_at, payload_json
               FROM connector_heartbeats
              ORDER BY last_seen_at DESC`,
          ).all() as Array<{
            connector_id: string
            status: string
            agent_version: string
            capabilities_json: string
            last_seen_at: string
            payload_json: string
          }>
          const connectors = rows.map((row) => {
            let payload: { inventory?: Record<string, unknown> } = {}
            try { payload = JSON.parse(row.payload_json) as typeof payload } catch { /* redacted */ }
            return {
              connector_id: row.connector_id,
              status: row.status,
              agent_version: row.agent_version,
              capabilities: JSON.parse(row.capabilities_json || '[]'),
              last_seen_at: row.last_seen_at,
              inventory: payload.inventory || {},
            }
          })
          return Response.json({ connectors })
        } catch (error) {
          const message = (error as Error).message || 'inventory unavailable'
          const status = message.includes('Unauthorized') ? 401 : message.includes('Forbidden') ? 403 : 500
          return Response.json({ error: message }, { status })
        }
      },
    },
  },
})
