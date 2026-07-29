// GET /api/unified/v1/operator/bootstrap — ArchGate Connect + UnifiedUI
// Bastion endpoints + catalog (no customer private IPs). ADR-008A.

import { createFileRoute } from '@tanstack/react-router'
import { buildOperatorBootstrap } from '@/server/unified-bff'
import { resolveOperatorSession } from '@/server/operator-session'
import { logger } from '@/server/logger'
import { unifiedCorsHeaders } from '@/server/unified-cors'

function corsHeaders(request: Request): Record<string, string> {
  return unifiedCorsHeaders(request, { methods: 'GET, OPTIONS' })
}

export const Route = createFileRoute('/api/unified/v1/operator/bootstrap')({
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
          const session = await resolveOperatorSession(request)
          const body = await buildOperatorBootstrap(session)
          return new Response(JSON.stringify(body), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized') ? 401 : 500
          logger.warn({ err: msg }, 'operator bootstrap failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
