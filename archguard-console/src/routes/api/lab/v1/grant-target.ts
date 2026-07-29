// POST /api/lab/v1/grant-target — lab/UAT smoke for Manager grant path.
// Gated by ARCHGATE_LAB=1 and still requires an authenticated session with
// grant permissions (use GET /test-login?u=testadmin first in lab).

import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from '@/server/session-guard'
import { runGrantPersonTarget } from '@/server/lifecycle-fn'
import { logger } from '@/server/logger'

function labEnabled(): boolean {
  return process.env.ARCHGATE_LAB === '1'
}

const bodySchema = z.object({
  username: z.string().min(1).max(128),
  target: z.string().min(1).max(128),
  role: z.string().max(128).optional(),
  ttl: z.string().max(32).optional(),
})

export const Route = createFileRoute('/api/lab/v1/grant-target')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        const headers = { 'Content-Type': 'application/json' }
        if (!labEnabled()) {
          return new Response(JSON.stringify({ error: 'lab API disabled' }), {
            status: 404,
            headers,
          })
        }
        try {
          const s = requireSession()
          requireAnyPerm(
            s,
            ['persons:update', 'gateways:manage', 'system:admin'],
            'persons:update',
          )
          const raw = await request.json().catch(() => ({}))
          const parsed = bodySchema.safeParse(raw)
          if (!parsed.success) {
            return new Response(
              JSON.stringify({ error: parsed.error.message }),
              { status: 400, headers },
            )
          }
          const result = await runGrantPersonTarget(
            parsed.data,
            sessionActor(s),
          )
          logger.info(
            {
              username: parsed.data.username,
              target: parsed.data.target,
              ok: result.ok,
            },
            'lab grant-target',
          )
          return new Response(JSON.stringify(result), {
            status: result.ok ? 200 : 502,
            headers,
          })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 500
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
