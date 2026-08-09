// GET /api/unified/v1/recordings/:name — authenticated recording playback.

import { createFileRoute } from '@tanstack/react-router'
import { fetchRustGuacRecording } from '@/server/rustguac-proxy'
import { resolveOperatorSession } from '@/server/operator-session'
import { requireAnyPerm } from '@/server/session-guard'
import { getBrokerSession } from '@/server/db'
import { unifiedCorsHeaders } from '@/server/unified-cors'

export const Route = createFileRoute('/api/unified/v1/recordings/$name')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, {
          status: 204,
          headers: unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' }),
        }),
      GET: async ({ request, params }) => {
        const headers = unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' })
        try {
          const session = await resolveOperatorSession(request)
          requireAnyPerm(session, ['gateways:read'], 'gateways:read')
          const match = params.name.match(/^([0-9a-f-]{36})\.guac$/i)
          if (!match || !getBrokerSession(match[1])) {
            return new Response(JSON.stringify({ error: 'recording not found' }), {
              status: 404,
              headers: { ...headers, 'Content-Type': 'application/json' },
            })
          }
          const upstream = await fetchRustGuacRecording(params.name)
          if (!upstream.ok) {
            return new Response(JSON.stringify({ error: 'recording not found' }), {
              status: upstream.status === 404 ? 404 : 502,
              headers: { ...headers, 'Content-Type': 'application/json' },
            })
          }
          const responseHeaders = new Headers(headers)
          responseHeaders.set(
            'Content-Type',
            upstream.headers.get('content-type') || 'application/octet-stream',
          )
          const disposition = upstream.headers.get('content-disposition')
          if (disposition) responseHeaders.set('Content-Disposition', disposition)
          return new Response(upstream.body, { status: 200, headers: responseHeaders })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : msg.includes('inválido')
                ? 400
                : 502
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers: { ...headers, 'Content-Type': 'application/json' },
          })
        }
      },
    },
  },
})
