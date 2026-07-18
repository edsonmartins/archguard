// UnifiedUI BFF — contracts in documentos/runbooks/unified-ui-bff-contracts.md
// REST handlers for SPA operator catalog (no admin tokens to browser).

import { listSites } from './sites'
import { listWarpgateTargets } from './warpgate-proxy'
import { listConnections as listGuacConnections } from './guacamole-proxy'
import {
  filterSitesByTenant,
  getSessionOrNull,
  sessionPermissions,
} from './session-guard'
import type { SessionData } from './auth'
import { deriveTenants } from '@/lib/auth/roles'
import { logger } from './logger'

export type UnifiedConnection = {
  id: string
  name: string
  site: string
  tenant: string
  protocol: string
  engine: 'warpgate' | 'guacamole'
  target: string
  description?: string
}

export type UnifiedSessionProfile = {
  display_name: string
  email: string
  username: string
  tenants: string[]
}

function sessionTenants(s: SessionData): string[] {
  return deriveTenants(s.groups || [])
}

/**
 * Build operator catalog from sites SoT + optional live WG/Guac names.
 * Never includes internal host IPs (ADR-008).
 */
export async function listUnifiedConnections(
  session: SessionData,
): Promise<UnifiedConnection[]> {
  const sites = filterSitesByTenant(await listSites(), session)
  const out: UnifiedConnection[] = []

  let wgNames = new Set<string>()
  try {
    const targets = await listWarpgateTargets()
    wgNames = new Set(targets.map((t) => t.name))
  } catch {
    /* gateway optional for catalog */
  }
  let guacNames = new Set<string>()
  try {
    const conns = await listGuacConnections()
    guacNames = new Set(conns.map((c) => c.name))
  } catch {
    /* optional */
  }

  const isAdmin = sessionPermissions(session).includes('system:admin')
  const groups = new Set((session.groups || []).map((g) => g.replace(/@.*$/, '')))

  for (const site of sites) {
    for (const t of site.targets || []) {
      const engine = t.engine === 'guacamole' ? 'guacamole' : 'warpgate'
      const protocol = (t.protocolo || 'ssh').toLowerCase()
      if (t.roles?.length && !isAdmin) {
        const ok = t.roles.some(
          (r) =>
            groups.has(r) ||
            groups.has(r.replace(/^tenant-/, 'tenant_')) ||
            groups.has(r.replace(/^tenant_/, 'tenant-')),
        )
        if (!ok) continue
      }
      const known =
        engine === 'warpgate' ? wgNames.has(t.nome) : guacNames.has(t.nome)
      out.push({
        id: `${site.slug}:${t.nome}`,
        name: t.nome,
        site: site.cliente,
        tenant: site.tenant_group,
        protocol: protocol.includes('postgres')
          ? 'postgres'
          : protocol.includes('http')
            ? 'http'
            : protocol.includes('rdp')
              ? 'rdp'
              : 'ssh',
        engine,
        target: t.nome,
        description: known
          ? t.notas || `${engine} · ${site.slug}`
          : t.notas || `${engine} · ${site.slug} (pending apply)`,
      })
    }
  }
  return out
}

export function profileFromSession(s: SessionData): UnifiedSessionProfile {
  return {
    display_name: s.user?.displayName || s.user?.name || 'operador',
    email: s.user?.email || '',
    username: s.user?.name || s.user?.email || '',
    tenants: sessionTenants(s),
  }
}

/** Create short-lived session metadata for Guacamole / launch (no admin token). */
export async function createUnifiedSession(
  session: SessionData,
  body: {
    connection_id?: string
    target?: string
    protocol?: string
  },
): Promise<{
  tunnel_url: string
  connect_data: string
  expires_in: number
  launch?: {
    engine: string
    target: string
    protocol: string
    warpgate_public?: string
  }
}> {
  const target = body.target || body.connection_id?.split(':').pop() || ''
  if (!target) {
    throw new Error('target or connection_id required')
  }

  const catalog = await listUnifiedConnections(session)
  const hit = catalog.find(
    (c) =>
      c.target === target ||
      c.id === body.connection_id ||
      c.id.endsWith(`:${target}`),
  )
  if (!hit) {
    throw new Error('Forbidden: connection not in catalog')
  }

  const expiresIn = 120
  const guacPublic =
    process.env.GUACAMOLE_PUBLIC_URL ||
    process.env.VITE_GUAC_URL ||
    'https://guac.archgate.com.br'
  const wgPublic =
    process.env.WARPGATE_PUBLIC_URL || 'https://wg.archgate.com.br'

  if (hit.engine === 'guacamole') {
    const opaque = Buffer.from(
      JSON.stringify({
        u: session.user?.name,
        t: hit.target,
        exp: Date.now() + expiresIn * 1000,
      }),
    ).toString('base64url')
    logger.info(
      { user: session.user?.name, target: hit.target },
      'unified session guacamole issued',
    )
    return {
      tunnel_url: guacPublic.replace(/\/$/, '') + '/websocket-tunnel',
      connect_data: opaque,
      expires_in: expiresIn,
      launch: {
        engine: 'guacamole',
        target: hit.target,
        protocol: hit.protocol,
      },
    }
  }

  return {
    tunnel_url: '',
    connect_data: '',
    expires_in: expiresIn,
    launch: {
      engine: 'warpgate',
      target: hit.target,
      protocol: hit.protocol,
      warpgate_public: wgPublic,
    },
  }
}

export function requireUnifiedSession(): SessionData {
  const s = getSessionOrNull()
  if (!s) throw new Error('Unauthorized')
  return s
}
