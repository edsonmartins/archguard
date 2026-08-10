// GET /api/org/v1/connectors/inventory/history — bounded inventory snapshots.

import { createFileRoute } from '@tanstack/react-router'
import { requireAnyPerm, requireSession } from '@/server/session-guard'
import { getDb } from '@/server/db'

export const Route = createFileRoute('/api/org/v1/connectors/inventory/history')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const session = requireSession()
          requireAnyPerm(session, ['sites:read', 'sites:update'], 'sites:read')
          const unrestricted = Boolean(
            session.permissions?.includes('system:admin') ||
            session.groups?.some((group) => group === 'archguard_super_admins' || group === 'system:admin'),
          )
          const connectorId = new URL(request.url).searchParams.get('connector_id') || ''
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
          if (connectorId && !unrestricted && !allowed.has(connectorId)) {
            return Response.json({ error: 'connector not found' }, { status: 404 })
          }
          const rows = (connectorId
            ? getDb().prepare(
                `SELECT connector_id, message_id, status, agent_version, observed_at, inventory_json
                   FROM connector_inventory_history
                  WHERE connector_id = ? ORDER BY observed_at DESC LIMIT 100`,
              ).all(connectorId)
            : getDb().prepare(
                `SELECT connector_id, message_id, status, agent_version, observed_at, inventory_json
                   FROM connector_inventory_history
                  ORDER BY observed_at DESC LIMIT 500`,
              ).all()) as Array<{
                connector_id: string
                message_id: string
                status: string
                agent_version: string
                observed_at: string
                inventory_json: string
              }>
          return Response.json({ history: rows.filter((row) => unrestricted || allowed.has(row.connector_id)).map((row) => ({
            connector_id: row.connector_id,
            message_id: row.message_id,
            status: row.status,
            agent_version: row.agent_version,
            observed_at: row.observed_at,
            inventory: JSON.parse(row.inventory_json || '{}'),
          })) })
        } catch (error) {
          const message = (error as Error).message || 'inventory history unavailable'
          const status = message.includes('Unauthorized') ? 401 : message.includes('Forbidden') ? 403 : 500
          return Response.json({ error: message }, { status })
        }
      },
    },
  },
})
