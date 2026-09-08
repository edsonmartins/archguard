// GET /api/unified/v1/recordings — server-side RustGuac recording catalog.

import { createFileRoute } from '@tanstack/react-router'
import { listRustGuacRecordings } from '@/server/rustguac-proxy'
import { resolveOperatorSession } from '@/server/operator-session'
import { requireAnyPerm } from '@/server/session-guard'
import { hasAnyPerm } from '@/server/session-guard'
import { listBrokerSessionsForPrincipal } from '@/server/db'
import { unifiedCorsHeaders } from '@/server/unified-cors'

export const Route = createFileRoute('/api/unified/v1/recordings')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, {
          status: 204,
          headers: unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' }),
        }),
      GET: async ({ request }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' }),
        }
        try {
          const session = await resolveOperatorSession(request)
          requireAnyPerm(session, ['gateways:read'], 'gateways:read')
          const all = await listRustGuacRecordings()
          const recordings = hasAnyPerm(session, ['system:admin'])
            ? all
            : all.filter((recording) => listBrokerSessionsForPrincipal(
              session.user?.name || session.user?.email || 'unknown',
            ).includes(recording.name.replace(/\.guac$/i, '')))
          return new Response(JSON.stringify({ recordings }), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 502
          return new Response(JSON.stringify({ error: msg }), { status, headers })
        }
      },
    },
  },
})
