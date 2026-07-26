// POST /api/org/v1/checkouts/:id/approve

import { createFileRoute } from '@tanstack/react-router'
import { getOrgAccount } from '@/server/org-accounts'
import {
  approveCheckout,
  getCheckout,
  samePrincipal,
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

export const Route = createFileRoute('/api/org/v1/checkouts/$id/approve')({
  server: {
    handlers: {
      POST: async ({ params }) => {
        const headers = { 'Content-Type': 'application/json' }
        try {
          const s = requireSession()
          requireAnyPerm(
            s,
            ['org_accounts:approve', 'org_accounts:admin'],
            'org_accounts:approve',
          )
          const actor = sessionActor(s)
          const existing = getCheckout(params.id)
          if (!existing || existing.status !== 'pending') {
            return new Response(
              JSON.stringify({ error: 'not pending' }),
              { status: 400, headers },
            )
          }
          if (samePrincipal(existing.principal, actor)) {
            return new Response(
              JSON.stringify({ error: 'self-approve forbidden' }),
              { status: 403, headers },
            )
          }
          const approved = approveCheckout(params.id, actor)
          if (!approved) {
            return new Response(JSON.stringify({ error: 'approve failed' }), {
              status: 400,
              headers,
            })
          }
          const acc = getOrgAccount(approved.account_id)
          let secret: Record<string, unknown> | undefined
          if (acc?.secret_ref && openbaoTokenConfigured()) {
            const fields = await readSecretData(acc.secret_ref)
            if (fields) {
              secret = {
                password:
                  fields.password ||
                  fields.value ||
                  fields.secret ||
                  fields.pass,
                username:
                  fields.username || fields.user || acc.login_hint || undefined,
                api_key: fields.api_key || fields.token || fields.key,
                fields,
              }
            }
          }
          recordActivity(
            'POST',
            `/archgate/org-accounts/checkout/${params.id}/approve`,
            actor,
            'success',
            undefined,
            {
              principal: approved.principal,
              account_slug: approved.account_slug,
            },
          )
          return new Response(
            JSON.stringify({ checkout: approved, secret }),
            { status: 200, headers },
          )
        } catch (e) {
          const msg = (e as Error).message || 'error'
          logger.warn({ err: msg }, 'approve checkout failed')
          return new Response(JSON.stringify({ error: msg }), {
            status: msg.includes('Self-approve') ? 403 : 400,
            headers,
          })
        }
      },
    },
  },
})
