import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { getSite, upsertSite } from './sites'
import {
  createRole,
  deleteRole,
  deleteTarget,
  listWarpgateRoles,
  listWarpgateTargets,
  upsertTarget,
  warpgateConfigured,
  warpgatePublicUrl,
  listWarpgateSessions,
  terminateWarpgateSession,
  type ApplyTargetInput,
} from './warpgate-proxy'
import {
  createConnection,
  guacamoleConfigured,
  listConnections,
} from './guacamole-proxy'
import { openbaoTokenConfigured, readSecretValue } from './openbao-proxy'
import {
  assertSiteTenantAccess,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { logger } from './logger'
import {
  filterByRoleNames,
  filterByTargetNames,
  gatewayTenantScope,
} from './gateway-scope'

function protocolToKind(protocolo: string): ApplyTargetInput['kind'] {
  const p = protocolo.toLowerCase()
  if (p.includes('postgres') || p === 'pg' || p === 'psql') return 'Postgres'
  if (p.includes('mysql') || p === 'mariadb') return 'MySql'
  if (p.includes('http') || p.includes('https')) return 'Http'
  return 'Ssh'
}

function protocolToGuac(
  protocolo: string,
): 'ssh' | 'rdp' | 'vnc' {
  const p = protocolo.toLowerCase()
  if (p.includes('rdp')) return 'rdp'
  if (p.includes('vnc')) return 'vnc'
  return 'ssh'
}

/**
 * Resolve target password without global lab fallbacks.
 * Priority:
 * 1. one-shot request map (passwords[name])
 * 2. OpenBao KV via target.secret_ref
 * 3. env TARGET_SECRET_<NAME>
 */
async function resolveTargetPassword(
  targetName: string,
  passwords?: Record<string, string>,
  secretRef?: string,
): Promise<string | undefined> {
  const fromRequest = passwords?.[targetName]
  if (fromRequest) return fromRequest

  if (secretRef?.trim()) {
    if (!openbaoTokenConfigured()) {
      throw new Error(
        `Target ${targetName}: secret_ref set but OPENBAO_APP_TOKEN missing`,
      )
    }
    try {
      const v = await readSecretValue(secretRef.trim())
      if (v) return v
      logger.warn(
        { targetName, secretRef },
        'secret_ref resolved empty from OpenBao',
      )
    } catch (e) {
      throw new Error(
        `Target ${targetName}: OpenBao secret_ref failed: ${(e as Error).message}`,
      )
    }
  }

  const envKey = `TARGET_SECRET_${targetName
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '_')}`
  return process.env[envKey] || undefined
}

function assertHttpTarget(t: {
  nome: string
  protocolo: string
  host: string
  port: number
}): void {
  const p = t.protocolo.toLowerCase()
  if (!p.includes('http')) return
  if (!t.host?.trim()) {
    throw new Error(`HTTP target ${t.nome}: host obrigatório`)
  }
  if (!t.port || t.port < 1) {
    throw new Error(`HTTP target ${t.nome}: port obrigatória`)
  }
}

const READ_PERMS = [
  'gateways:read',
  'gateways:manage',
  'sites:update',
] as const

const MANAGE_PERMS = ['gateways:manage'] as const

const APPLY_PERMS = ['gateways:manage', 'sites:update'] as const

export const getWarpgateStatusFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    requireSession()
    return {
      configured: warpgateConfigured(),
      /** Browser stock UI (public DNS) */
      url: warpgatePublicUrl(),
    }
  },
)

export const listWarpgateTargetsFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, [...READ_PERMS], 'gateways:read')
    const scope = await gatewayTenantScope(s)
    const targets = await listWarpgateTargets()
    const mapped = targets.map((t) => ({
      id: t.id,
      name: t.name,
      description: t.description,
      kind: (t.options as { kind?: string } | undefined)?.kind,
      host: (t.options as { host?: string } | undefined)?.host,
      port: (t.options as { port?: number } | undefined)?.port,
    }))
    return filterByTargetNames(mapped, scope)
  },
)

export const listWarpgateRolesFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, [...READ_PERMS], 'gateways:read')
    const scope = await gatewayTenantScope(s)
    const roles = await listWarpgateRoles()
    return filterByRoleNames(roles, scope)
  },
)

/**
 * Materialize site targets on gateways (ADR-009):
 * - engine warpgate → Warpgate admin API
 * - engine guacamole → Guacamole connections
 */
export const applySiteTargetsFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        /** Optional one-shot passwords by target name (not persisted). */
        passwords: z.record(z.string()).optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...APPLY_PERMS], 'gateways:manage')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    if (!site.targets.length) throw new Error('Site sem targets para aplicar')

    const results: {
      name: string
      id: string
      engine: 'warpgate' | 'guacamole'
      ok: boolean
      error?: string
    }[] = []

    for (const t of site.targets) {
      const engine = t.engine === 'guacamole' ? 'guacamole' : 'warpgate'
      try {
        assertHttpTarget(t)
        if (engine === 'guacamole') {
          if (!guacamoleConfigured()) {
            throw new Error(
              'Guacamole não configurado (GUACAMOLE_ADMIN_PASSWORD)',
            )
          }
          const password = await resolveTargetPassword(
            t.nome,
            data.passwords,
            t.secret_ref,
          )
          const existing = (await listConnections()).find(
            (c) => c.name === t.nome,
          )
          if (existing) {
            results.push({
              name: t.nome,
              id: existing.id,
              engine: 'guacamole',
              ok: true,
            })
            continue
          }
          const created = await createConnection({
            name: t.nome,
            protocol: protocolToGuac(t.protocolo),
            hostname: t.host,
            port: t.port,
            username:
              t.username ||
              (protocolToGuac(t.protocolo) === 'ssh' ? 'labuser' : undefined),
            password,
          })
          results.push({
            name: t.nome,
            id: created.id,
            engine: 'guacamole',
            ok: true,
          })
        } else {
          if (!warpgateConfigured()) {
            throw new Error(
              'Warpgate não configurado (WARPGATE_ADMIN_PASSWORD)',
            )
          }
          const password = await resolveTargetPassword(
            t.nome,
            data.passwords,
            t.secret_ref,
          )
          const kind = protocolToKind(t.protocolo)
          const username =
            t.username ||
            (kind === 'Ssh' ? 'labuser' : undefined)
          const applied = await upsertTarget({
            name: t.nome,
            description: `Site ${site.cliente} (${site.slug})`,
            kind,
            host: t.host,
            port: t.port,
            username,
            password,
            roles: t.roles?.length ? t.roles : site.warpgate_roles,
          })
          results.push({
            name: t.nome,
            id: applied.id,
            engine: 'warpgate',
            ok: true,
          })
        }
      } catch (e) {
        results.push({
          name: t.nome,
          id: '',
          engine,
          ok: false,
          error: (e as Error).message,
        })
      }
    }

    const allOk = results.every((r) => r.ok)
    if (allOk) {
      await upsertSite(
        {
          ...site,
          inventariado: true,
        },
        sessionActor(s),
      )
    }

    return {
      results,
      allOk,
      warpgate: results.filter((r) => r.engine === 'warpgate'),
      guacamole: results.filter((r) => r.engine === 'guacamole'),
    }
  })

const targetFormSchema = z.object({
  name: z.string().min(1).max(128),
  description: z.string().optional(),
  kind: z.enum(['Ssh', 'Postgres', 'MySql', 'Http']),
  host: z.string().min(1),
  port: z.number().int().positive(),
  username: z.string().optional(),
  password: z.string().optional(),
  database: z.string().optional(),
  roles: z.array(z.string()).default([]),
})

export const upsertWarpgateTargetFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = targetFormSchema.safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    return upsertTarget(data)
  })

export const deleteWarpgateTargetFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    await deleteTarget(data.id)
    return { ok: true }
  })

export const createWarpgateRoleFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        name: z.string().min(1).max(128),
        description: z.string().optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    return createRole(data.name, data.description)
  })

export const deleteWarpgateRoleFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    await deleteRole(data.id)
    return { ok: true }
  })

/** Active sessions / tickets (W-C3). */
export const listWarpgateSessionsFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, [...READ_PERMS], 'gateways:read')
    if (!warpgateConfigured()) {
      return { configured: false, sessions: [] as Awaited<ReturnType<typeof listWarpgateSessions>> }
    }
    try {
      const sessions = await listWarpgateSessions()
      return { configured: true, sessions }
    } catch (e) {
      return {
        configured: true,
        sessions: [] as Awaited<ReturnType<typeof listWarpgateSessions>>,
        error: (e as Error).message,
      }
    }
  },
)

export const terminateWarpgateSessionFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    const result = await terminateWarpgateSession(data.id)
    if (!result.ok) throw new Error(result.detail)
    return result
  })
