// CP — Módulo Plataforma: saúde da stack ArchGate + meta control plane + runbooks.
// Probes são best-effort a partir do container do console (sem Docker socket).

import { createServerFn } from '@tanstack/react-start'
import { lookup } from 'node:dns/promises'
import { requireAnyPerm, requireSession } from './session-guard'
import { integrationFetch } from './http-integration-client'
import {
  getHealth as openbaoHealth,
  getSealStatus,
  openbaoAddr,
  openbaoConfigured,
  openbaoTokenConfigured,
  openbaoTokenKind,
} from './openbao-proxy'
import {
  listWarpgateRoles,
  listWarpgateTargets,
  warpgateConfigured,
  warpgateConnectInfo,
  warpgatePublicUrl,
} from './warpgate-proxy'
import {
  guacamoleConfigured,
  listConnections,
} from './guacamole-proxy'
import {
  axisConnectionInfo,
  listProprietarios,
  mentorsAxisMode,
} from './mentors-axis-proxy'
import { listSites, sitesBackend } from './sites'
import { kanidmAdminConfigured } from './kanidm-admin'
import { pingDb } from './db'

export type PlatformServiceStatus = 'ok' | 'degraded' | 'error' | 'unreachable' | 'unconfigured'

export type PlatformService = {
  id: string
  name: string
  group: 'identity' | 'gateway' | 'secrets' | 'connectivity' | 'control_plane'
  status: PlatformServiceStatus
  detail?: string
  endpoint?: string
  latency_ms?: number
  version?: string
}

export type PlatformRunbook = {
  id: string
  title: string
  description: string
  href: string
  external?: boolean
}

const KANIDM_URL = (
  process.env.ARCHGUARD_ID_URL || 'https://id.archgate.com.br'
).replace(/\/$/, '')

function envEndpoint(key: string, fallback: string): string {
  return (process.env[key] || fallback).replace(/\/$/, '')
}

async function timedProbe(
  fn: () => Promise<{
    status: PlatformServiceStatus
    detail?: string
    version?: string
  }>,
): Promise<{
  status: PlatformServiceStatus
  detail?: string
  version?: string
  latency_ms: number
}> {
  const t0 = Date.now()
  try {
    const r = await fn()
    return { ...r, latency_ms: Date.now() - t0 }
  } catch (e) {
    return {
      status: 'unreachable',
      detail: (e as Error).message.slice(0, 200),
      latency_ms: Date.now() - t0,
    }
  }
}

async function probeKanidm(): Promise<PlatformService> {
  const endpoint = KANIDM_URL
  const r = await timedProbe(async () => {
    // Prefer public status; fall back to domain root
    const paths = ['/status', '/v1/system/status', '/']
    let lastErr = 'no response'
    for (const p of paths) {
      try {
        const res = await integrationFetch(`${endpoint}${p}`, {
          method: 'GET',
          integration: 'kanidm',
          timeoutMs: 5_000,
        })
        if (res.ok || res.status === 401 || res.status === 403) {
          // 401 on authenticated paths still means service is up
          const text = await res.text().catch(() => '')
          let version: string | undefined
          try {
            const j = JSON.parse(text) as { version?: string }
            version = j.version
          } catch {
            /* ignore */
          }
          return {
            status: 'ok',
            detail: `HTTP ${res.status}${p === '/' ? ' (root)' : ` ${p}`}`,
            version,
          }
        }
        lastErr = `HTTP ${res.status} ${p}`
      } catch (e) {
        lastErr = (e as Error).message
      }
    }
    return { status: 'error', detail: lastErr }
  })
  return {
    id: 'kanidm',
    name: 'Kanidm (ArchGuard ID)',
    group: 'identity',
    endpoint,
    ...r,
  }
}

async function probeOpenBao(): Promise<PlatformService> {
  const endpoint = openbaoAddr()
  if (!openbaoConfigured()) {
    return {
      id: 'openbao',
      name: 'OpenBao',
      group: 'secrets',
      status: 'unconfigured',
      detail: 'OPENBAO_ADDR ausente',
      endpoint,
    }
  }
  const r = await timedProbe(async () => {
    const health = await openbaoHealth()
    let sealDetail = ''
    try {
      const seal = await getSealStatus()
      sealDetail = seal.sealed ? 'sealed' : 'unsealed'
    } catch {
      sealDetail = 'seal unknown'
    }
    if (health.sealed || sealDetail === 'sealed') {
      return {
        status: 'degraded',
        detail: `sealed; token=${openbaoTokenKind()}`,
        version: health.version,
      }
    }
    return {
      status: 'ok',
      detail: `${sealDetail}; token=${openbaoTokenConfigured() ? openbaoTokenKind() : 'none'}`,
      version: health.version,
    }
  })
  return {
    id: 'openbao',
    name: 'OpenBao',
    group: 'secrets',
    endpoint,
    ...r,
  }
}

async function probeWarpgate(): Promise<PlatformService> {
  const info = warpgateConnectInfo()
  const endpoint = info.public_url
  if (!warpgateConfigured()) {
    return {
      id: 'warpgate',
      name: 'Warpgate',
      group: 'gateway',
      status: 'unconfigured',
      detail: 'WARPGATE_ADMIN_PASSWORD ausente',
      endpoint,
    }
  }
  const r = await timedProbe(async () => {
    let inventory = ''
    try {
      const [targets, roles] = await Promise.all([
        listWarpgateTargets(),
        listWarpgateRoles(),
      ])
      inventory = `targets=${targets.length}; roles=${roles.length}`
      return {
        status: 'ok' as const,
        detail: `admin API OK via ${info.connect_host} (SNI ${info.host_header}); ${inventory}`,
      }
    } catch (e) {
      return {
        status: 'error' as const,
        detail: `admin API: ${(e as Error).message.slice(0, 160)} (connect=${info.connect_host})`,
      }
    }
  })
  return {
    id: 'warpgate',
    name: 'Warpgate',
    group: 'gateway',
    endpoint,
    ...r,
  }
}

async function probeGuacamole(): Promise<PlatformService> {
  const endpoint = envEndpoint(
    'GUACAMOLE_URL',
    'http://archgate-gw_guacamole:8080',
  )
  const publicUrl = envEndpoint(
    'GUACAMOLE_PUBLIC_URL',
    'https://guac.archgate.com.br',
  )
  if (!guacamoleConfigured()) {
    return {
      id: 'guacamole',
      name: 'Guacamole',
      group: 'gateway',
      status: 'unconfigured',
      detail: 'GUACAMOLE_ADMIN_PASSWORD ausente',
      endpoint: publicUrl,
    }
  }
  const r = await timedProbe(async () => {
    const res = await integrationFetch(`${endpoint}/`, {
      method: 'GET',
      integration: 'guacamole',
      timeoutMs: 5_000,
    })
    if (!(res.status > 0)) {
      return { status: 'error' as const, detail: 'sem resposta' }
    }
    let inventory = ''
    try {
      const conns = await listConnections()
      inventory = `; connections=${conns.length}`
    } catch (e) {
      inventory = `; inventário: ${(e as Error).message.slice(0, 80)}`
    }
    return {
      status: 'ok' as const,
      detail: `API configurada; HTTP ${res.status}${inventory}`,
    }
  })
  return {
    id: 'guacamole',
    name: 'Guacamole',
    group: 'gateway',
    endpoint: publicUrl,
    ...r,
  }
}

async function probeAxis(): Promise<PlatformService> {
  const info = axisConnectionInfo()
  const mode = mentorsAxisMode()
  if (mode === 'disabled') {
    return {
      id: 'mentors_axis',
      name: 'Mentors Axis',
      group: 'control_plane',
      status: 'unconfigured',
      detail: 'MENTORS_AXIS_URL / MOCK ausentes',
    }
  }
  if (mode === 'mock') {
    return {
      id: 'mentors_axis',
      name: 'Mentors Axis',
      group: 'control_plane',
      status: 'degraded',
      detail: 'mode=mock (fixture local)',
      endpoint: info.url || undefined,
    }
  }
  const r = await timedProbe(async () => {
    const url = (info.url || '').replace(/\/$/, '')
    if (!url) return { status: 'error' as const, detail: 'URL vazia' }
    const res = await integrationFetch(`${url}/`, {
      method: 'GET',
      integration: 'axis',
      timeoutMs: 5_000,
    })
    if (
      !(
        res.ok ||
        res.status === 401 ||
        res.status === 403 ||
        res.status === 404
      )
    ) {
      return { status: 'error' as const, detail: `HTTP ${res.status}` }
    }
    const text = await res.text().catch(() => '')
    const isStub = text.includes('axis-stub')
    let inventory = ''
    try {
      const props = await listProprietarios(0, 50)
      inventory = `; proprietarios=${props.length}`
    } catch (e) {
      inventory = `; list: ${(e as Error).message.slice(0, 80)}`
    }
    return {
      status: 'ok' as const,
      detail: `live${isStub ? ' (stub)' : ' (api)'}; HTTP ${res.status}; auth=${info.auth}${inventory}`,
    }
  })
  return {
    id: 'mentors_axis',
    name: 'Mentors Axis',
    group: 'control_plane',
    endpoint: info.url || undefined,
    ...r,
  }
}

async function probeConnectorLab(): Promise<PlatformService> {
  const names = [
    'archgate-site-piloto_connector',
    'archgate-site-piloto_ovpn-srv',
  ]
  const r = await timedProbe(async () => {
    const found: string[] = []
    for (const n of names) {
      try {
        const a = await lookup(n)
        found.push(`${n}→${a.address}`)
      } catch {
        /* not running */
      }
    }
    if (found.length === 0) {
      return {
        status: 'degraded',
        detail: 'piloto DNS não resolve (ok se stack VPN lab parada)',
      }
    }
    return { status: 'ok', detail: found.join('; ') }
  })
  return {
    id: 'connector_lab',
    name: 'Connector piloto (lab)',
    group: 'connectivity',
    ...r,
  }
}

function controlPlaneMetaService(): PlatformService {
  const backend = sitesBackend()
  return {
    id: 'sites_sot',
    name: 'Sites SoT',
    group: 'control_plane',
    status: backend === 'postgres' ? 'ok' : 'degraded',
    detail:
      backend === 'postgres'
        ? 'PostgreSQL (CONSOLE_DATABASE_URL)'
        : 'SQLite fallback (volume local)',
  }
}

function consoleService(): PlatformService {
  return {
    id: 'console',
    name: 'ArchGuard Console',
    group: 'control_plane',
    status: 'ok',
    detail: `NODE_ENV=${process.env.NODE_ENV || 'undefined'}; lab=${process.env.ARCHGATE_LAB || '0'}; kanidm_sa=${kanidmAdminConfigured() ? 'yes' : 'no'}`,
    endpoint: envEndpoint(
      'CONSOLE_PUBLIC_URL',
      'https://console.archgate.com.br',
    ),
  }
}

const RUNBOOKS: PlatformRunbook[] = [
  {
    id: 'endpoints',
    title: 'Endpoints staging',
    description: 'DNS, portas Warpgate/Guac/Console',
    href: 'https://console.archgate.com.br',
    external: false,
  },
  {
    id: 'operator',
    title: 'Cliente operador multi-OS',
    description: 'SSH/Postgres via bastion (ADR-008)',
    href: 'https://wg.archgate.com.br',
    external: true,
  },
  {
    id: 'sites',
    title: 'Admin clientes / sites',
    description: 'CRUD sites, stack VPN, sync gateways',
    href: '/sites',
  },
  {
    id: 'gateways',
    title: 'Gateways',
    description: 'Targets Warpgate e conexões Guacamole',
    href: '/gateways',
  },
  {
    id: 'secrets',
    title: 'Segredos OpenBao',
    description: 'Seal, JWT, leases, unseal lab',
    href: '/secrets',
  },
  {
    id: 'axis',
    title: 'Mentors Axis sync',
    description: 'Proprietários → sites + tenant Kanidm',
    href: '/integrations/mentors-axis',
  },
  {
    id: 'id',
    title: 'Kanidm (stock)',
    description: 'Issuer OIDC / identidade',
    href: 'https://id.archgate.com.br',
    external: true,
  },
  {
    id: 'guac',
    title: 'Guacamole (stock)',
    description: 'Sessões browser Parte A',
    href: 'https://guac.archgate.com.br',
    external: true,
  },
]

export const getPlatformOverviewFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['settings:read', 'sites:read', 'secrets:read', 'system:admin'],
      'settings:read',
    )

    const [
      kanidm,
      openbao,
      warpgate,
      guacamole,
      axis,
      connector,
    ] = await Promise.all([
      probeKanidm(),
      probeOpenBao(),
      probeWarpgate(),
      probeGuacamole(),
      probeAxis(),
      probeConnectorLab(),
    ])

    const sitesSot = controlPlaneMetaService()
    const consoleSvc = consoleService()

    let sitesCount = 0
    try {
      sitesCount = (await listSites()).length
    } catch {
      sitesCount = -1
    }

    const services: PlatformService[] = [
      consoleSvc,
      sitesSot,
      kanidm,
      openbao,
      warpgate,
      guacamole,
      axis,
      connector,
    ]

    const summary = {
      ok: services.filter((x) => x.status === 'ok').length,
      degraded: services.filter((x) => x.status === 'degraded').length,
      error: services.filter(
        (x) => x.status === 'error' || x.status === 'unreachable',
      ).length,
      unconfigured: services.filter((x) => x.status === 'unconfigured').length,
      total: services.length,
    }

    const endpoints = {
      kanidm: KANIDM_URL,
      console: envEndpoint(
        'CONSOLE_PUBLIC_URL',
        'https://console.archgate.com.br',
      ),
      warpgate: warpgatePublicUrl(),
      warpgate_ssh: process.env.WARPGATE_SSH_ENDPOINT || 'wg.archgate.com.br:2222',
      warpgate_pg:
        process.env.WARPGATE_PG_ENDPOINT || 'wg.archgate.com.br:55432',
      guacamole: envEndpoint(
        'GUACAMOLE_PUBLIC_URL',
        'https://guac.archgate.com.br',
      ),
      openbao: openbaoAddr(),
      mentors_axis: axisConnectionInfo().url || null,
    }

    // Inventory counts (best-effort, for summary cards)
    let warpgateTargets = -1
    let guacamoleConnections = -1
    let axisProprietarios = -1
    try {
      if (warpgateConfigured()) {
        warpgateTargets = (await listWarpgateTargets()).length
      }
    } catch {
      /* ignore */
    }
    try {
      if (guacamoleConfigured()) {
        guacamoleConnections = (await listConnections()).length
      }
    } catch {
      /* ignore */
    }
    try {
      if (mentorsAxisMode() !== 'disabled') {
        axisProprietarios = (await listProprietarios(0, 200)).length
      }
    } catch {
      /* ignore */
    }

    // Touch SQLite so activity_log schema is created even if sites use Postgres.
    const activitySqliteOk = pingDb()

    return {
      generated_at: new Date().toISOString(),
      lab: process.env.ARCHGATE_LAB === '1',
      sites_backend: sitesBackend(),
      sites_count: sitesCount,
      activity_sqlite_ok: activitySqliteOk,
      inventory: {
        warpgate_targets: warpgateTargets,
        guacamole_connections: guacamoleConnections,
        axis_proprietarios: axisProprietarios,
      },
      summary,
      services,
      endpoints,
      runbooks: RUNBOOKS,
    }
  },
)
