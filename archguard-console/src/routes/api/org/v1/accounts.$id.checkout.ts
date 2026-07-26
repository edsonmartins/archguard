// POST /api/org/v1/accounts/:id/checkout — OCB-1 reveal with reason+TTL

import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { getOrgAccount } from '@/server/org-accounts'
import {
  TTL_DEFAULT,
  TTL_MAX,
  TTL_MIN,
  createCheckout,
} from '@/server/org-checkouts'
import {
  openbaoTokenConfigured,
  readSecretData,
} from '@/server/openbao-proxy'
import { recordActivity } from '@/server/activity-log'
import { logger } from '@/server/logger'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from '@/server/session-guard'

const bodySchema = z.object({
  reason: z.string().min(3).max(1000),
  ttl_seconds: z.number().int().min(TTL_MIN).max(TTL_MAX).optional(),
})

export const Route = createFileRoute('/api/org/v1/accounts/$id/checkout')({
  server: {
    handlers: {
      POST: async ({ request, params }) => {
        const headers = { 'Content-Type': 'application/json' }
        try {
          const s = requireSession()
          requireAnyPerm(
            s,
            ['org_accounts:checkout', 'org_accounts:admin'],
            'org_accounts:checkout',
          )
          const id = params.id
          const acc = getOrgAccount(id)
          if (!acc) {
            return new Response(JSON.stringify({ error: 'Not found' }), {
              status: 404,
              headers,
            })
          }
          const raw = await request.json().catch(() => ({}))
          const parsed = bodySchema.safeParse(raw)
          if (!parsed.success) {
            return new Response(
              JSON.stringify({ error: parsed.error.message }),
              { status: 400, headers },
            )
          }
          const actor = sessionActor(s)
          const ttl = parsed.data.ttl_seconds ?? TTL_DEFAULT
          const checkout = createCheckout({
            account_id: acc.id,
            account_slug: acc.slug,
            principal: actor,
            reason: parsed.data.reason.trim(),
            ttl_seconds: ttl,
            status: 'active',
          })

          if (!acc.secret_ref || !openbaoTokenConfigured()) {
            recordActivity(
              'POST',
              `/archgate/org-accounts/${acc.slug}/checkout`,
              actor,
              'error',
              !acc.secret_ref ? 'secret_ref missing' : 'openbao missing',
              { checkout_id: checkout.id },
            )
            return new Response(
              JSON.stringify({
                checkout,
                openbao_configured: openbaoTokenConfigured(),
                message: !acc.secret_ref
                  ? 'secret_ref missing'
                  : 'OPENBAO_APP_TOKEN missing',
              }),
              { status: 200, headers },
            )
          }

          const fields = await readSecretData(acc.secret_ref)
          if (!fields) {
            recordActivity(
              'POST',
              `/archgate/org-accounts/${acc.slug}/checkout`,
              actor,
              'error',
              'secret not found',
              { checkout_id: checkout.id },
            )
            return new Response(
              JSON.stringify({
                checkout,
                openbao_configured: true,
                message: `secret not found at ${acc.secret_ref}`,
              }),
              { status: 200, headers },
            )
          }

          recordActivity(
            'POST',
            `/archgate/org-accounts/${acc.slug}/checkout`,
            actor,
            'success',
            undefined,
            {
              checkout_id: checkout.id,
              reason: parsed.data.reason,
              ttl_seconds: ttl,
              secret_keys: Object.keys(fields),
            },
          )

          return new Response(
            JSON.stringify({
              checkout,
              openbao_configured: true,
              secret: {
                password:
                  fields.password ||
                  fields.value ||
                  fields.secret ||
                  fields.pass,
                username:
                  fields.username || fields.user || acc.login_hint || undefined,
                api_key: fields.api_key || fields.token || fields.key,
                fields,
              },
            }),
            { status: 200, headers },
          )
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 400
          logger.warn({ err: msg }, 'org checkout failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
