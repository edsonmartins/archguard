import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import {
  getHealth,
  getJwtConfig,
  getSealStatus,
  listAuthMethods,
  listDbLeases,
  listMounts,
  listPolicies,
  openbaoAddr,
  openbaoConfigured,
  openbaoTokenConfigured,
  openbaoTokenKind,
  revokeLease,
  unsealWithEnvKey,
} from './openbao-proxy'
import { requireAnyPerm, requireSession } from './session-guard'

export const getOpenBaoStatusFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    // vault:read is the sidebar entry for /vault; secrets:* for /secrets
    requireAnyPerm(
      s,
      ['secrets:read', 'secrets:manage', 'vault:read', 'vault:admin', 'system:admin'],
      'secrets:read',
    )
    if (!openbaoConfigured()) {
      return {
        configured: false,
        token_configured: false,
        token_kind: openbaoTokenKind(),
        addr: openbaoAddr(),
        health: null,
        seal: null,
      }
    }
    let health = null
    let seal = null
    try {
      health = await getHealth()
    } catch (e) {
      return {
        configured: true,
        token_configured: openbaoTokenConfigured(),
        token_kind: openbaoTokenKind(),
        addr: openbaoAddr(),
        health: null,
        seal: null,
        error: (e as Error).message,
      }
    }
    try {
      seal = await getSealStatus()
    } catch {
      /* optional */
    }
    return {
      configured: true,
      token_configured: openbaoTokenConfigured(),
      token_kind: openbaoTokenKind(),
      addr: openbaoAddr(),
      health,
      seal,
    }
  },
)

export const getOpenBaoOverviewFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, ['secrets:read', 'secrets:manage'], 'secrets:read')
    if (!openbaoTokenConfigured()) {
      throw new Error(
        'OPENBAO token não configurado no console (use OPENBAO_APP_TOKEN)',
      )
    }
    const [mounts, auth, policies, jwt, dbLeases] = await Promise.all([
      listMounts(),
      listAuthMethods(),
      listPolicies(),
      getJwtConfig(),
      listDbLeases('lab-readonly').catch(() => ({
        role: 'lab-readonly',
        lease_ids: [] as string[],
      })),
    ])
    return {
      mounts,
      auth,
      policies,
      jwt: jwt as Record<
        string,
        string | number | boolean | null | object
      > | null,
      db_leases: dbLeases,
      token_kind: openbaoTokenKind(),
    }
  },
)

export const unsealOpenBaoFn = createServerFn({ method: 'POST' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, ['secrets:manage'], 'secrets:manage')
    return unsealWithEnvKey()
  },
)

export const revokeOpenBaoLeaseFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ lease_id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['secrets:manage'], 'secrets:manage')
    await revokeLease(data.lease_id)
    return { ok: true }
  })
