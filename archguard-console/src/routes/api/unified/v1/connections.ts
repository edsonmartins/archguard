// GET /api/unified/v1/connections — UnifiedUI catalog (BFF)

import { createFileRoute } from '@tanstack/react-router'
import { listUnifiedConnections } from '@/server/unified-bff'
import { resolveOperatorSession } from '@/server/operator-session'
import { logger } from '@/server/logger'
import { unifiedCorsHeaders } from '@/server/unified-cors'

function corsHeaders(request: Request): Record<string, string> {
  return unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' })
}

export const Route = createFileRoute('/api/unified/v1/connections')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, { status: 204, headers: corsHeaders(request) }),
      GET: async ({ request }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...corsHeaders(request),
        }
        try {
          // Cookie (browser) or Bearer OIDC (Connect desktop) — same as sessions.
          const session = await resolveOperatorSession(request)
          const connections = await listUnifiedConnections(session)
          return new Response(JSON.stringify({ connections }), {
            status: 200,
            headers,
          })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized') ? 401 : 500
          logger.warn({ err: msg }, 'unified connections failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
