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
import {
  TTL_DEFAULT,
  TTL_MAX,
  TTL_MIN,
  checkinCheckout,
  createCheckout,
  listCheckoutsForAccount,
  type OrgCheckout,
} from './org-checkouts'
import {
  openbaoTokenConfigured,
  readSecretData,
  writeSecretData,
} from './openbao-proxy'
import { recordActivity } from './activity-log'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'

export type CheckoutResult = {
  checkout: OrgCheckout
  /** Present only when status=active and secret resolved */
  secret?: {
    password?: string
    username?: string
    api_key?: string
    fields: Record<string, string>
  }
  message?: string
  openbao_configured: boolean
}

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

/**
 * OCB-1: checkout secret with reason + TTL.
 * Dual-control hard gate is OCB-2 — for now P0 is allowed with audit flag.
 */
export const checkoutOrgAccountFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        id: z.string().min(1),
        reason: z.string().min(3).max(1000),
        ttl_seconds: z.number().int().min(TTL_MIN).max(TTL_MAX).optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<CheckoutResult> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:checkout', 'org_accounts:admin'],
      'org_accounts:checkout',
    )
    const actor = sessionActor(s)
    const acc = getOrgAccount(data.id)
    if (!acc) throw new Error('Conta não encontrada')

    const ttl = data.ttl_seconds ?? TTL_DEFAULT
    const reason = data.reason.trim()
    if (reason.length < 3) throw new Error('reason required (min 3 chars)')

    const baoOk = openbaoTokenConfigured()
    const checkout = createCheckout({
      account_id: acc.id,
      account_slug: acc.slug,
      principal: actor,
      reason,
      ttl_seconds: ttl,
      status: 'active',
    })

    if (!acc.secret_ref) {
      recordActivity(
        'POST',
        `/archgate/org-accounts/${acc.slug}/checkout`,
        actor,
        'error',
        'secret_ref missing',
        { checkout_id: checkout.id, reason },
      )
      return {
        checkout,
        openbao_configured: baoOk,
        message:
          'Conta sem secret_ref. Admin deve preencher o path OpenBao (ex. secret/data/org/store/…).',
      }
    }

    if (!baoOk) {
      recordActivity(
        'POST',
        `/archgate/org-accounts/${acc.slug}/checkout`,
        actor,
        'error',
        'openbao token missing',
        { checkout_id: checkout.id, reason },
      )
      return {
        checkout,
        openbao_configured: false,
        message:
          'OpenBao sem OPENBAO_APP_TOKEN no console. Configure wire de secrets (OCB-1).',
      }
    }

    try {
      const fields = await readSecretData(acc.secret_ref)
      if (!fields) {
        recordActivity(
          'POST',
          `/archgate/org-accounts/${acc.slug}/checkout`,
          actor,
          'error',
          'secret empty or not found',
          { checkout_id: checkout.id, reason, secret_ref: acc.secret_ref },
        )
        return {
          checkout,
          openbao_configured: true,
          message: `Secret não encontrado em ${acc.secret_ref}. Grave o valor no OpenBao (admin).`,
        }
      }

      recordActivity(
        'POST',
        `/archgate/org-accounts/${acc.slug}/checkout`,
        actor,
        'success',
        undefined,
        {
          checkout_id: checkout.id,
          reason,
          ttl_seconds: ttl,
          expires_at: checkout.expires_at,
          dual_control_required: acc.requires_dual_control,
          // never log secret material
          secret_keys: Object.keys(fields),
        },
      )

      return {
        checkout,
        openbao_configured: true,
        secret: {
          password: fields.password || fields.value || fields.secret || fields.pass,
          username: fields.username || fields.user || acc.login_hint || undefined,
          api_key: fields.api_key || fields.token || fields.key,
          fields,
        },
        message: acc.requires_dual_control
          ? 'Checkout ativo (P0). Dual-control formal chega no OCB-2 — uso auditado.'
          : 'Checkout ativo. Não compartilhe no chat; faça check-in ao terminar.',
      }
    } catch (e) {
      const msg = (e as Error).message
      recordActivity(
        'POST',
        `/archgate/org-accounts/${acc.slug}/checkout`,
        actor,
        'error',
        msg,
        { checkout_id: checkout.id, reason },
      )
      throw e
    }
  })

export const checkinOrgAccountFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ checkout_id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<{ checkout: OrgCheckout | null }> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:checkout', 'org_accounts:admin'],
      'org_accounts:checkout',
    )
    const actor = sessionActor(s)
    const c = checkinCheckout(data.checkout_id, actor)
    recordActivity(
      'POST',
      `/archgate/org-accounts/checkout/${data.checkout_id}/checkin`,
      actor,
      c ? 'success' : 'error',
      c ? undefined : 'not found',
      { status: c?.status },
    )
    return { checkout: c }
  })

export const listOrgCheckoutsFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ account_id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<OrgCheckout[]> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:read', 'org_accounts:admin', 'org_accounts:checkout'],
      'org_accounts:read',
    )
    return listCheckoutsForAccount(data.account_id)
  })

/** Admin: write secret material to OpenBao at account secret_ref (never stored in SQLite). */
export const storeOrgAccountSecretFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        id: z.string().min(1),
        password: z.string().max(4096).optional(),
        username: z.string().max(256).optional(),
        api_key: z.string().max(8192).optional(),
        note: z.string().max(1000).optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    if (!r.data.password && !r.data.api_key) {
      throw new Error('Informe password ou api_key')
    }
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['org_accounts:admin'], 'org_accounts:admin')
    if (!openbaoTokenConfigured()) {
      throw new Error('OpenBao sem OPENBAO_APP_TOKEN no console')
    }
    const acc = getOrgAccount(data.id)
    if (!acc) throw new Error('Conta não encontrada')
    let ref = acc.secret_ref
    if (!ref) {
      ref = `secret/data/org/${acc.category}/${acc.slug}`
      upsertOrgAccount(
        {
          slug: acc.slug,
          name: acc.name,
          category: acc.category,
          product: acc.product,
          url: acc.url,
          login_hint: acc.login_hint,
          auth_kind: acc.auth_kind,
          secret_ref: ref,
          criticality: acc.criticality,
          owner_group: acc.owner_group,
          requires_dual_control: acc.requires_dual_control,
          notes: acc.notes,
          runbook_url: acc.runbook_url,
        },
        sessionActor(s),
      )
    }
    const fields: Record<string, string> = {}
    if (data.password) fields.password = data.password
    if (data.username) fields.username = data.username
    if (data.api_key) fields.api_key = data.api_key
    if (data.note) fields.note = data.note
    const written = await writeSecretData(ref, fields)
    const actor = sessionActor(s)
    recordActivity(
      'PUT',
      `/archgate/org-accounts/${acc.slug}/secret`,
      actor,
      'success',
      undefined,
      { secret_ref: written.secret_ref, keys: Object.keys(fields) },
    )
    return { ok: true as const, secret_ref: written.secret_ref }
  })
