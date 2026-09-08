import { createFileRoute } from '@tanstack/react-router'
import { resolveOperatorSession } from '@/server/operator-session'
import { closeBrokerSessionAndLease } from '@/server/broker-session'
import { unifiedCorsHeaders } from '@/server/unified-cors'
import { getBrokerSession } from '@/server/db'
import { hasAnyPerm } from '@/server/session-guard'

export const Route = createFileRoute('/api/unified/v1/sessions/$sessionId')({
  server: {
    handlers: {
      DELETE: async ({ request, params }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...unifiedCorsHeaders(request, { methods: 'DELETE, OPTIONS' }),
        }
        try {
          const session = await resolveOperatorSession(request)
          const broker = getBrokerSession(params.sessionId)
          const principal = session.user?.name || session.user?.email || 'unknown'
          if (!broker) throw new Error('Forbidden: sessão não encontrada')
          if (!hasAnyPerm(session, ['system:admin']) && broker.principal !== principal) {
            throw new Error('Forbidden: sessão fora do escopo do operador')
          }
          await closeBrokerSessionAndLease(params.sessionId)
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
