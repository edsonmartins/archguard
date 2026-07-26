// UnifiedUI BFF — contracts in documentos/runbooks/unified-ui-bff-contracts.md
// REST handlers for SPA operator catalog (no admin tokens to browser).

import { listSites } from './sites'
import { listWarpgateTargets } from './warpgate-proxy'
import {
  listConnections as listGuacConnections,
  issueOperatorGuacTunnel,
} from './guacamole-proxy'
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
  /** Normalize tenant_rio_quality ↔ tenant-rio-quality (Kanidm vs Warpgate roles). */
  const norm = (g: string) => g.replace(/@.*$/, '').replace(/-/g, '_')
  const groups = new Set((session.groups || []).map(norm))

  for (const site of sites) {
    for (const t of site.targets || []) {
      const engine = t.engine === 'guacamole' ? 'guacamole' : 'warpgate'
      const protocol = (t.protocolo || 'ssh').toLowerCase()
      if (t.roles?.length && !isAdmin) {
        const ok = t.roles.some((r) => {
          const n = norm(r)
          return (
            groups.has(n) ||
            groups.has(n.replace(/^tenant_/, '')) ||
            groups.has(`tenant_${n}`)
          )
        })
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
          : protocol.includes('mysql') || protocol.includes('mariadb')
            ? 'mysql'
            : protocol.includes('http')
              ? 'http'
              : protocol.includes('rdp')
                ? 'rdp'
                : protocol.includes('vnc')
                  ? 'vnc'
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
  /** In-app browser surface (Guac SSO, Oracle UI, HTTP). */
  embed_url?: string
  embed_mode?: 'websocket' | 'iframe'
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

  const proto = (hit.protocol || body.protocol || '').toLowerCase()
  const wantsGuac =
    hit.engine === 'guacamole' ||
    proto === 'rdp' ||
    proto === 'vnc' ||
    proto === 'kubernetes' // guac when browser path

  if (wantsGuac) {
    const base = guacPublic.replace(/\/$/, '')
    const username =
      session.user?.name || session.user?.email?.split('@')[0] || ''
    // Public WS for guacamole-common-js (token in connect_data). oauth2-proxy
    // must skip auth on /websocket-tunnel so short-lived Guac tokens work
    // (deploy/guacamole/oauth2-proxy-deploy.sh).
    const tunnelWs = base.replace(/^http/, 'ws') + '/websocket-tunnel'

    try {
      const guac = await issueOperatorGuacTunnel(username, hit.target)
      const embedWithToken =
        `${base}/?token=${encodeURIComponent(guac.authToken)}#/client/${guac.clientHash}`
      logger.info(
        {
          user: username,
          target: hit.target,
          protocol: proto,
          guac_id: guac.connectionId,
          mode: 'websocket',
        },
        'unified session guacamole token issued',
      )
      return {
        tunnel_url: tunnelWs,
        connect_data: guac.connectData,
        expires_in: expiresIn,
        embed_url: embedWithToken,
        embed_mode: 'websocket' as const,
        launch: {
          engine: 'guacamole',
          target: hit.target,
          protocol: hit.protocol || guac.protocol || proto || 'rdp',
        },
      }
    } catch (e) {
      // Fallback: iframe SSO portal (no admin token leak).
      const msg = e instanceof Error ? e.message : String(e)
      logger.warn(
        { user: username, target: hit.target, err: msg },
        'unified session guacamole token mint failed — iframe fallback',
      )
      return {
        tunnel_url: '',
        connect_data: '',
        expires_in: expiresIn,
        embed_url: base + '/',
        embed_mode: 'iframe' as const,
        launch: {
          engine: 'guacamole',
          target: hit.target,
          protocol: hit.protocol || proto || 'rdp',
        },
      }
    }
  }

  // HTTP / Oracle UI / other: in-app browser surface URL (no internal IPs).
  if (
    proto === 'http' ||
    proto === 'https' ||
    proto === 'oracle' ||
    hit.target.toLowerCase().includes('oracle')
  ) {
    const consoleBase =
      process.env.CONSOLE_PUBLIC_URL || 'https://console.archgate.com.br'
    const embed =
      proto === 'oracle' || hit.target.toLowerCase().includes('oracle')
        ? `${consoleBase.replace(/\/$/, '')}/oracle`
        : `${wgPublic.replace(/\/$/, '')}/`
    return {
      tunnel_url: '',
      connect_data: '',
      expires_in: expiresIn,
      embed_url: embed,
      embed_mode: 'iframe' as const,
      launch: {
        engine: hit.engine || 'warpgate',
        target: hit.target,
        protocol: hit.protocol || proto,
        warpgate_public: wgPublic,
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

/** Bastion endpoints for ArchGate Connect / UnifiedUI launch (no internal target IPs). */
export type OperatorBastion = {
  ssh_host: string
  ssh_port: number
  pg_host: string
  pg_port: number
  pg_database: string
  mysql_host: string
  mysql_port: number
  http_base: string
  guac_base: string
  console_base: string
  oracle_ui: string
}

export type OperatorBootstrap = {
  bastion: OperatorBastion
  profile: UnifiedSessionProfile
  connections: UnifiedConnection[]
}

export function operatorBastionFromEnv(): OperatorBastion {
  const n = (k: string, d: number) => {
    const v = process.env[k]
    if (!v) return d
    const x = Number(v)
    return Number.isFinite(x) ? x : d
  }
  // TCP 2222/55432 are published on the VPS IP; CF only fronts HTTPS for wg.*
  return {
    ssh_host:
      process.env.WG_SSH_HOST ||
      process.env.WARPGATE_SSH_HOST ||
      process.env.ARCHGATE_VPS_IP ||
      '217.196.60.108',
    ssh_port: n('WG_SSH_PORT', 2222),
    pg_host:
      process.env.WG_PG_HOST ||
      process.env.WARPGATE_PG_HOST ||
      process.env.ARCHGATE_VPS_IP ||
      '217.196.60.108',
    pg_port: n('WG_PG_PORT', 55432),
    pg_database: process.env.WG_PG_DB || 'archgate_lab',
    mysql_host:
      process.env.WG_MYSQL_HOST ||
      process.env.WARPGATE_MYSQL_HOST ||
      process.env.ARCHGATE_VPS_IP ||
      '217.196.60.108',
    mysql_port: n('WG_MYSQL_PORT', 33306),
    http_base:
      process.env.WG_HTTP_BASE ||
      process.env.WARPGATE_PUBLIC_URL ||
      'https://wg.archgate.com.br',
    guac_base: process.env.GUAC_PUBLIC_URL || 'https://guac.archgate.com.br',
    console_base: process.env.CONSOLE_PUBLIC_URL || 'https://console.archgate.com.br',
    oracle_ui:
      process.env.ORACLE_UI_URL ||
      `${process.env.CONSOLE_PUBLIC_URL || 'https://console.archgate.com.br'}/oracle`,
  }
}

export async function buildOperatorBootstrap(
  session: SessionData,
): Promise<OperatorBootstrap> {
  return {
    bastion: operatorBastionFromEnv(),
    profile: profileFromSession(session),
    connections: await listUnifiedConnections(session),
  }
}
