// POST /api/unified/v1/recordings/:name/retention — configure recording retention.

import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { getRecordingRetention, setRecordingRetention } from '@/server/rustguac-proxy'
import { resolveOperatorSession } from '@/server/operator-session'
import { requireAnyPerm, sessionActor } from '@/server/session-guard'
import { recordActivity } from '@/server/activity-log'
import { unifiedCorsHeaders } from '@/server/unified-cors'

const bodySchema = z.object({
  legal_hold: z.boolean(),
  retain_until: z.string().datetime({ offset: true }).nullable().optional(),
})

export const Route = createFileRoute('/api/unified/v1/recordings/$name/retention')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) => new Response(null, {
        status: 204,
        headers: unifiedCorsHeaders(request, { methods: 'POST, OPTIONS' }),
      }),
      POST: async ({ request, params }) => {
        const headers = { 'Content-Type': 'application/json', ...unifiedCorsHeaders(request, { methods: 'POST, OPTIONS' }) }
        try {
          const session = await resolveOperatorSession(request)
          requireAnyPerm(session, ['gateways:manage', 'system:admin'], 'gateways:manage')
          if (!/^[0-9a-f-]{36}\.guac$/i.test(params.name)) {
            return new Response(JSON.stringify({ error: 'recording not found' }), { status: 404, headers })
          }
          const parsed = bodySchema.safeParse(await request.json().catch(() => ({})))
          if (!parsed.success) return new Response(JSON.stringify({ error: parsed.error.message }), { status: 400, headers })
          const retainUntil = parsed.data.retain_until === undefined ? getRecordingRetention(params.name).retain_until : parsed.data.retain_until
          const retention = setRecordingRetention({
            name: params.name,
            legal_hold: parsed.data.legal_hold,
            retain_until: retainUntil,
            updated_by: sessionActor(session),
          })
          recordActivity('POST', `/api/unified/v1/recordings/${params.name}/retention`, sessionActor(session), 'success', undefined, retention)
          return new Response(JSON.stringify({ recording: params.name, retention }), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized') ? 401 : msg.includes('Forbidden') ? 403 : 500
          return new Response(JSON.stringify({ error: msg }), { status, headers })
        }
      },
    },
  },
})
