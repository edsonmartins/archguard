// POST /api/org/v1/connectors/heartbeat — connector mTLS transport endpoint.

import { createFileRoute } from '@tanstack/react-router'
import { recordConnectorHeartbeat, type ConnectorHeartbeat } from '@/server/connector-heartbeat'

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
