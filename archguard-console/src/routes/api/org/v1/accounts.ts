// GET/POST /api/org/v1/accounts — Org Credential Broker (ADR-013 metadata)

import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import {
  listOrgAccounts,
  backfillOrgAccountFederation,
  seedDefaultOrgAccountsIfEmpty,
  upsertOrgAccount,
} from '@/server/org-accounts'
import { recordActivity } from '@/server/activity-log'
import { logger } from '@/server/logger'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from '@/server/session-guard'

const bodySchema = z.object({
  slug: z.string().min(1).max(128),
  name: z.string().min(1).max(256),
  category: z.enum(['cloud', 'store', 'product_admin', 'vendor', 'other']),
  product: z.string().max(128).optional(),
  url: z.string().max(512).optional(),
  login_hint: z.string().max(256).optional(),
  auth_kind: z
    .enum(['password', 'api_key', 'oidc', 'totp_password'])
    .optional(),
  secret_ref: z.string().max(512).optional(),
  criticality: z.enum(['P0', 'P1', 'P2']).optional(),
  owner_group: z.string().max(128).optional(),
  requires_dual_control: z.boolean().optional(),
  notes: z.string().max(4000).optional(),
  runbook_url: z.string().max(512).optional(),
})

export const Route = createFileRoute('/api/org/v1/accounts')({
  server: {
    handlers: {
      GET: async () => {
        const headers = { 'Content-Type': 'application/json' }
        try {
          const s = requireSession()
          requireAnyPerm(
            s,
            ['org_accounts:read', 'org_accounts:admin'],
            'org_accounts:read',
          )
          const actor = sessionActor(s)
          seedDefaultOrgAccountsIfEmpty(actor)
          backfillOrgAccountFederation(actor)
          const accounts = listOrgAccounts()
          return new Response(JSON.stringify({ accounts }), {
            status: 200,
            headers,
          })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 500
          logger.warn({ err: msg }, 'org accounts list failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
      POST: async ({ request }) => {
        const headers = { 'Content-Type': 'application/json' }
        try {
          const s = requireSession()
          requireAnyPerm(s, ['org_accounts:admin'], 'org_accounts:admin')
          const raw = await request.json().catch(() => ({}))
          const parsed = bodySchema.safeParse(raw)
          if (!parsed.success) {
            return new Response(
              JSON.stringify({ error: parsed.error.message }),
              { status: 400, headers },
            )
          }
          const actor = sessionActor(s)
          const acc = upsertOrgAccount(parsed.data, actor)
          recordActivity(
            'POST',
            `/archgate/org-accounts/${acc.slug}`,
            actor,
            'success',
            undefined,
            { slug: acc.slug, name: acc.name },
          )
          return new Response(JSON.stringify(acc), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized')
            ? 401
            : msg.includes('Forbidden')
              ? 403
              : 400
          logger.warn({ err: msg }, 'org accounts upsert failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
