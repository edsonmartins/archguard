// POST /api/org/v1/connectors/heartbeat — connector mTLS transport endpoint.

import { createFileRoute } from '@tanstack/react-router'
import { recordConnectorHeartbeat, type ConnectorHeartbeat } from '@/server/connector-heartbeat'
import { getDb } from '@/server/db'

export const Route = createFileRoute('/api/org/v1/connectors/heartbeat')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        // These headers must be injected by the trusted mTLS reverse proxy.
        // Direct/public requests are rejected by default.
        if (process.env.CONNECTOR_MTLS_TRUSTED_PROXY !== '1') {
          return Response.json({ error: 'connector mTLS transport unavailable' }, { status: 503 })
        }
        if (request.headers.get('x-connector-mtls-verified') !== 'SUCCESS') {
          return Response.json({ error: 'client certificate not verified' }, { status: 401 })
        }
        const certificateConnector = request.headers.get('x-connector-id') || ''
        if (process.env.CONNECTOR_MTLS_REQUIRE_CERT_REGISTRY === '1') {
          const serial = request.headers.get('x-connector-cert-serial') || ''
          if (!serial) return Response.json({ error: 'certificate serial missing' }, { status: 401 })
          const cert = getDb().prepare(
            'SELECT connector_id, status FROM connector_certificates WHERE serial_number = ?',
          ).get(serial) as { connector_id: string; status: string } | undefined
          if (!cert || cert.connector_id !== certificateConnector || cert.status !== 'active') {
            return Response.json({ error: 'certificate revoked or unknown' }, { status: 401 })
          }
        }
        try {
          const body = (await request.json()) as ConnectorHeartbeat
          if (!certificateConnector || certificateConnector !== body.connector_id) {
            return Response.json({ error: 'certificate/connector mismatch' }, { status: 403 })
          }
          recordConnectorHeartbeat(body)
          return Response.json({ accepted: true, connector_id: body.connector_id })
        } catch (e) {
          return Response.json({ error: (e as Error).message || 'invalid heartbeat' }, { status: 400 })
        }
      },
    },
  },
})
