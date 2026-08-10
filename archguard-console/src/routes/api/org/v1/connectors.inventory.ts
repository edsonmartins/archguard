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
          const unrestricted = Boolean(
            session.permissions?.includes('system:admin') ||
            session.groups?.some((group) => group === 'archguard_super_admins' || group === 'system:admin'),
          )
          const siteRows = getDb().prepare(
            'SELECT tenant_group, connector_id, connectors_json FROM sites',
          ).all() as Array<{ tenant_group: string; connector_id: string | null; connectors_json: string }>
          const allowed = new Set<string>()
          for (const site of siteRows) {
            if (!unrestricted && !session.groups?.includes(site.tenant_group)) continue
            if (site.connector_id) allowed.add(site.connector_id)
            try {
              const connectors = JSON.parse(site.connectors_json || '[]') as Array<{ id?: string }>
              for (const connector of connectors) if (connector.id) allowed.add(connector.id)
            } catch { /* malformed inventory is ignored */ }
          }
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
          const connectors = rows.filter((row) => unrestricted || allowed.has(row.connector_id)).map((row) => {
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
