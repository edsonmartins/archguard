// GET /api/unified/v1/connections — UnifiedUI catalog (BFF)

import { createFileRoute } from '@tanstack/react-router'
import {
  listUnifiedConnections,
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
    'Access-Control-Allow-Headers':
      'Content-Type, Authorization, X-ArchGate-User, X-ArchGate-Tenants',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  }
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
          const session = requireUnifiedSession()
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
