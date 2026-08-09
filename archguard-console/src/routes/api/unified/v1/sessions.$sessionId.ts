import { createFileRoute } from '@tanstack/react-router'
import { resolveOperatorSession } from '@/server/operator-session'
import { closeRustGuacSession } from '@/server/rustguac-proxy'
import { unifiedCorsHeaders } from '@/server/unified-cors'

export const Route = createFileRoute('/api/unified/v1/sessions/$sessionId')({
  server: {
    handlers: {
      DELETE: async ({ request, params }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...unifiedCorsHeaders(request, { methods: 'DELETE, OPTIONS' }),
        }
        try {
          await resolveOperatorSession(request)
          await closeRustGuacSession(params.sessionId)
          return new Response(JSON.stringify({ ok: true }), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized') ? 401 : msg.includes('Forbidden') ? 403 : 400
          return new Response(JSON.stringify({ error: msg }), { status, headers })
        }
      },
      OPTIONS: async ({ request }) =>
        new Response(null, {
          status: 204,
          headers: unifiedCorsHeaders(request, { methods: 'DELETE, OPTIONS' }),
        }),
    },
  },
})
