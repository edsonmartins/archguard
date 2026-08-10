// GET /api/org/v1/connectors/diagnostics — bounded, redacted connector diagnostics.

import { createFileRoute } from '@tanstack/react-router'
import { requireAnyPerm, requireSession } from '@/server/session-guard'
import { getDb } from '@/server/db'

type SiteRow = { tenant_group: string; connector_id: string | null; connectors_json: string }

function allowedConnectors(session: ReturnType<typeof requireSession>): { unrestricted: boolean; ids: Set<string> } {
  const unrestricted = Boolean(
    session.permissions?.includes('system:admin') ||
    session.groups?.some((group) => group === 'archguard_super_admins' || group === 'system:admin'),
  )
  const ids = new Set<string>()
  const sites = getDb().prepare('SELECT tenant_group, connector_id, connectors_json FROM sites').all() as SiteRow[]
  for (const site of sites) {
    if (!unrestricted && !session.groups?.includes(site.tenant_group)) continue
    if (site.connector_id) ids.add(site.connector_id)
    try {
      const connectors = JSON.parse(site.connectors_json || '[]') as Array<{ id?: string }>
      for (const connector of connectors) if (connector.id) ids.add(connector.id)
    } catch { /* malformed inventory is ignored */ }
  }
  return { unrestricted, ids }
}

export const Route = createFileRoute('/api/org/v1/connectors/diagnostics')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const session = requireSession()
          requireAnyPerm(session, ['sites:read', 'sites:update'], 'sites:read')
          const scope = allowedConnectors(session)
          const params = new URL(request.url).searchParams
          const requested = params.get('connector_id') || ''
          if (requested && !scope.unrestricted && !scope.ids.has(requested)) {
            return Response.json({ error: 'connector not found' }, { status: 404 })
          }
          const heartbeats = (requested
            ? getDb().prepare(
                `SELECT connector_id, status, agent_version, capabilities_json, last_seen_at
                   FROM connector_heartbeats WHERE connector_id = ?`,
              ).all(requested)
            : getDb().prepare(
                `SELECT connector_id, status, agent_version, capabilities_json, last_seen_at
                   FROM connector_heartbeats ORDER BY last_seen_at DESC LIMIT 500`,
              ).all()) as Array<{
                connector_id: string
                status: string
                agent_version: string
                capabilities_json: string
                last_seen_at: string
              }>
          const now = Date.now()
          const staleAfter = Number(process.env.CONNECTOR_DIAGNOSTIC_STALE_SECONDS || 180) * 1000
          const diagnostics = heartbeats
            .filter((row) => scope.unrestricted || scope.ids.has(row.connector_id))
            .map((row) => {
              const age = Math.max(0, now - Date.parse(row.last_seen_at))
              let capabilities: string[] = []
              try { capabilities = JSON.parse(row.capabilities_json || '[]') as string[] } catch { /* redacted */ }
              const state = row.status === 'revoked' ? 'revoked' : age > staleAfter ? 'stale' : row.status
              const certs = getDb().prepare(
                `SELECT serial_number, status, issued_at, revoked_at
                   FROM connector_certificates WHERE connector_id = ? ORDER BY issued_at DESC LIMIT 5`,
              ).all(row.connector_id) as Array<{ serial_number: string; status: string; issued_at: string; revoked_at: string | null }>
              return {
                connector_id: row.connector_id,
                state,
                heartbeat_status: row.status,
                last_seen_at: row.last_seen_at,
                age_seconds: Math.floor(age / 1000),
                agent_version: row.agent_version,
                capabilities,
                certificates: certs.map((cert) => ({
                  serial_suffix: cert.serial_number.slice(-12),
                  status: cert.status,
                  issued_at: cert.issued_at,
                  revoked_at: cert.revoked_at,
                })),
              }
            })
          return Response.json({ stale_after_seconds: Math.floor(staleAfter / 1000), diagnostics })
        } catch (error) {
          const message = (error as Error).message || 'connector diagnostics unavailable'
          const status = message.includes('Unauthorized') ? 401 : message.includes('Forbidden') ? 403 : 500
          return Response.json({ error: message }, { status })
        }
      },
    },
  },
})
