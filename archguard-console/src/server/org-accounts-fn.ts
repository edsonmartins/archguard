import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import type { OrgAccount, OrgAccountInput } from '@/lib/api/types/org-account'
import {
  deleteOrgAccount,
  getOrgAccount,
  listOrgAccounts,
  seedDefaultOrgAccountsIfEmpty,
  upsertOrgAccount,
} from './org-accounts'
import { recordActivity } from './activity-log'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'

const inputSchema = z.object({
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

export const listOrgAccountsFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<OrgAccount[]> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:read', 'org_accounts:admin'],
      'org_accounts:read',
    )
    seedDefaultOrgAccountsIfEmpty(sessionActor(s))
    return listOrgAccounts()
  },
)

export const getOrgAccountFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<OrgAccount> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:read', 'org_accounts:admin'],
      'org_accounts:read',
    )
    const acc = getOrgAccount(data.id)
    if (!acc) throw new Error('Not found')
    return acc
  })

export const upsertOrgAccountFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = inputSchema.safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data as OrgAccountInput
  })
  .handler(async ({ data }): Promise<OrgAccount> => {
    const s = requireSession()
    requireAnyPerm(s, ['org_accounts:admin'], 'org_accounts:admin')
    const actor = sessionActor(s)
    try {
      const acc = upsertOrgAccount(data, actor)
      recordActivity(
        'PUT',
        `/archgate/org-accounts/${acc.slug}`,
        actor,
        'success',
        undefined,
        {
          slug: acc.slug,
          name: acc.name,
          criticality: acc.criticality,
          // never log secret values
          secret_ref: acc.secret_ref ? '[set]' : '',
        },
      )
      return acc
    } catch (e) {
      recordActivity(
        'PUT',
        `/archgate/org-accounts/${data.slug}`,
        actor,
        'error',
        (e as Error).message,
      )
      throw e
    }
  })

export const deleteOrgAccountFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<{ ok: boolean }> => {
    const s = requireSession()
    requireAnyPerm(s, ['org_accounts:admin'], 'org_accounts:admin')
    const actor = sessionActor(s)
    const ok = deleteOrgAccount(data.id)
    recordActivity(
      'DELETE',
      `/archgate/org-accounts/${data.id}`,
      actor,
      ok ? 'success' : 'error',
      ok ? undefined : 'not found',
    )
    return { ok }
  })
