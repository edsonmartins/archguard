import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import type { OrgAccount, OrgAccountInput } from '@/lib/api/types/org-account'
import {
  deleteOrgAccount,
  getOrgAccount,
  listOrgAccounts,
  markOrgAccountRotated,
  seedDefaultOrgAccountsIfEmpty,
  upsertOrgAccount,
} from './org-accounts'
import { notifyPendingCheckout } from './org-notify'
import {
  TTL_DEFAULT,
  TTL_MAX,
  TTL_MIN,
  approveCheckout,
  checkinCheckout,
  createCheckout,
  denyCheckout,
  getCheckout,
  listCheckoutsForAccount,
  listPendingCheckouts,
  samePrincipal,
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
  federation_status: z
    .enum([
      'password_only',
      'oidc_primary',
      'oidc_only',
      'api_key_primary',
      'external_idp',
    ])
    .optional(),
  oidc_client_id: z.string().max(128).optional(),
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

async function revealForAccount(
  acc: NonNullable<ReturnType<typeof getOrgAccount>>,
  checkout: OrgCheckout,
  actor: string,
  reason: string,
): Promise<CheckoutResult> {
  const baoOk = openbaoTokenConfigured()
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
        'OpenBao sem OPENBAO_APP_TOKEN no console. Configure wire de secrets.',
    }
  }
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
      ttl_seconds: checkout.ttl_seconds,
      expires_at: checkout.expires_at,
      dual_control: acc.requires_dual_control,
      approved_by: checkout.approved_by,
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
    message:
      'Checkout ativo. Não compartilhe no chat; faça check-in ao terminar.',
  }
}

/**
 * Checkout with reason + TTL.
 * OCB-2: accounts with requires_dual_control create status=pending (no secret)
 * until a different principal approves.
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

    const needsDual = !!acc.requires_dual_control
    const baoOk = openbaoTokenConfigured()

    if (needsDual) {
      const checkout = createCheckout({
        account_id: acc.id,
        account_slug: acc.slug,
        principal: actor,
        reason,
        ttl_seconds: ttl,
        status: 'pending',
      })
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
          status: 'pending',
          dual_control: true,
        },
      )
      void notifyPendingCheckout({
        checkout_id: checkout.id,
        account_slug: acc.slug,
        account_name: acc.name,
        principal: actor,
        reason,
        ttl_seconds: ttl,
        criticality: acc.criticality,
      })
      return {
        checkout,
        openbao_configured: baoOk,
        message:
          'Checkout P0 pendente de dual-control. Outro admin com org_accounts:approve deve aprovar na fila.',
      }
    }

    const checkout = createCheckout({
      account_id: acc.id,
      account_slug: acc.slug,
      principal: actor,
      reason,
      ttl_seconds: ttl,
      status: 'active',
    })
    try {
      return await revealForAccount(acc, checkout, actor, reason)
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

/** OCB-2: approve pending checkout and reveal secret to the approver flow (returned once). */
export const approveOrgCheckoutFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ checkout_id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<CheckoutResult> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:approve', 'org_accounts:admin'],
      'org_accounts:approve',
    )
    const actor = sessionActor(s)
    const existing = getCheckout(data.checkout_id)
    if (!existing) throw new Error('Checkout não encontrado')
    if (existing.status !== 'pending') {
      throw new Error(`Checkout não está pendente (status=${existing.status})`)
    }
    if (samePrincipal(existing.principal, actor)) {
      recordActivity(
        'POST',
        `/archgate/org-accounts/checkout/${data.checkout_id}/approve`,
        actor,
        'error',
        'self-approve forbidden',
      )
      throw new Error('Forbidden: self-approve não permitido')
    }

    let approved: OrgCheckout | null
    try {
      approved = approveCheckout(data.checkout_id, actor)
    } catch (e) {
      recordActivity(
        'POST',
        `/archgate/org-accounts/checkout/${data.checkout_id}/approve`,
        actor,
        'error',
        (e as Error).message,
      )
      throw e
    }
    if (!approved || approved.status !== 'active') {
      throw new Error('Falha ao aprovar checkout')
    }

    const acc = getOrgAccount(approved.account_id)
    if (!acc) throw new Error('Conta não encontrada')

    recordActivity(
      'POST',
      `/archgate/org-accounts/checkout/${data.checkout_id}/approve`,
      actor,
      'success',
      undefined,
      {
        principal: approved.principal,
        account_slug: approved.account_slug,
        reason: approved.reason,
      },
    )

    // Secret returned to approver session — they hand process offline / requester
    // re-checks via their own follow-up is not re-revealed without new checkout.
    // Spec: reveal after approve — return secret once to the approver who can copy
    // and coordinate; requester was told "pending".
    return await revealForAccount(acc, approved, actor, approved.reason)
  })

export const denyOrgCheckoutFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        checkout_id: z.string().min(1),
        note: z.string().max(500).optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<{ checkout: OrgCheckout | null }> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:approve', 'org_accounts:admin'],
      'org_accounts:approve',
    )
    const actor = sessionActor(s)
    const existing = getCheckout(data.checkout_id)
    if (!existing) throw new Error('Checkout não encontrado')
    if (existing.status !== 'pending') {
      throw new Error(`Checkout não está pendente (status=${existing.status})`)
    }
    if (samePrincipal(existing.principal, actor)) {
      throw new Error(
        'Use Check-in para cancelar seu próprio pedido pendente',
      )
    }
    const denied = denyCheckout(data.checkout_id, actor)
    recordActivity(
      'POST',
      `/archgate/org-accounts/checkout/${data.checkout_id}/deny`,
      actor,
      'success',
      undefined,
      {
        principal: existing.principal,
        account_slug: existing.account_slug,
        note: data.note || '',
      },
    )
    return { checkout: denied }
  })

export const listPendingOrgCheckoutsFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<OrgCheckout[]> => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['org_accounts:approve', 'org_accounts:admin'],
      'org_accounts:approve',
    )
    return listPendingCheckouts()
  },
)

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
        /** When true, mark rotated_at and keep previous value note in OpenBao field rotated_from. */
        rotate: z.boolean().optional(),
        auth_kind: z
          .enum(['password', 'api_key', 'oidc', 'totp_password'])
          .optional(),
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
    const actor = sessionActor(s)
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
          auth_kind: data.auth_kind || acc.auth_kind,
          secret_ref: ref,
          criticality: acc.criticality,
          owner_group: acc.owner_group,
          requires_dual_control: acc.requires_dual_control,
          notes: acc.notes,
          runbook_url: acc.runbook_url,
        },
        actor,
      )
    } else if (data.auth_kind && data.auth_kind !== acc.auth_kind) {
      upsertOrgAccount(
        {
          slug: acc.slug,
          name: acc.name,
          category: acc.category,
          product: acc.product,
          url: acc.url,
          login_hint: acc.login_hint,
          auth_kind: data.auth_kind,
          secret_ref: acc.secret_ref,
          criticality: acc.criticality,
          owner_group: acc.owner_group,
          requires_dual_control: acc.requires_dual_control,
          notes: acc.notes,
          runbook_url: acc.runbook_url,
        },
        actor,
      )
    }

    const fields: Record<string, string> = {}
    if (data.password) fields.password = data.password
    if (data.username) fields.username = data.username
    if (data.api_key) fields.api_key = data.api_key
    if (data.note) fields.note = data.note
    if (data.rotate) {
      fields.rotated_at = new Date().toISOString()
      fields.rotated_by = actor
    }
    const written = await writeSecretData(ref, fields)
    if (data.rotate) {
      markOrgAccountRotated(acc.id, actor)
    }
    recordActivity(
      'PUT',
      `/archgate/org-accounts/${acc.slug}/secret`,
      actor,
      'success',
      undefined,
      {
        secret_ref: written.secret_ref,
        keys: Object.keys(fields).filter(
          (k) => !['password', 'api_key'].includes(k),
        ),
        rotated: !!data.rotate,
        auth_kind: data.auth_kind || acc.auth_kind,
      },
    )
    return {
      ok: true as const,
      secret_ref: written.secret_ref,
      rotated: !!data.rotate,
    }
  })
