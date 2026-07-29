// POST /api/unified/v1/sessions — short-lived tunnel / launch metadata
// Cookie (browser) or Bearer (ArchGate Connect desktop) via resolveOperatorSession.

import { createFileRoute } from '@tanstack/react-router'
import { createUnifiedSession } from '@/server/unified-bff'
import { resolveOperatorSession } from '@/server/operator-session'
import { logger } from '@/server/logger'
import { unifiedCorsHeaders } from '@/server/unified-cors'

function corsHeaders(request: Request): Record<string, string> {
  return unifiedCorsHeaders(request, { methods: 'POST, OPTIONS' })
}

export const Route = createFileRoute('/api/unified/v1/sessions')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, { status: 204, headers: corsHeaders(request) }),
      POST: async ({ request }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...corsHeaders(request),
        }
        try {
          const session = await resolveOperatorSession(request)
          const body = (await request.json().catch(() => ({}))) as {
            connection_id?: string
            target?: string
            protocol?: string
          }
          const result = await createUnifiedSession(session, body)
          return new Response(JSON.stringify(result), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 400
          logger.warn({ err: msg }, 'unified sessions failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})