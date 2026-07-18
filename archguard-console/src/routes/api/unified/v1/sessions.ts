// POST /api/unified/v1/sessions — short-lived tunnel / launch metadata

import { createFileRoute } from '@tanstack/react-router'
import {
  createUnifiedSession,
  requireUnifiedSession,
} from '@/server/unified-bff'
import { logger } from '@/server/logger'

function corsHeaders(request: Request): HeadersInit {
  const origin = request.headers.get('Origin') || ''
  const allow =
    process.env.UNIFIED_UI_ORIGIN ||
    process.env.CORS_ALLOW_ORIGIN ||
    origin ||
    '*'
  return {
    'Access-Control-Allow-Origin': allow === '*' ? '*' : allow,
    'Access-Control-Allow-Credentials': 'true',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  }
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
          const session = requireUnifiedSession()
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
