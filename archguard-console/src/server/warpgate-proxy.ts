// Server-side Warpgate admin API client (secrets never reach the browser).
// Auth cookie is cached briefly to avoid login-per-request storms.
//
// Lab/Swarm note: public hostname (wg.archgate.com.br) often does NOT resolve
// inside containers. Use WARPGATE_CONNECT_HOST (e.g. archgate-edge_traefik) as
// TCP peer and WARPGATE_HOST as Host/SNI so Traefik routes + session cookies work.

import https from 'node:https'
import { logger } from './logger'
import { getCachedAuth, invalidateAuthCache } from './http-integration-client'

/** Public URL shown in UI / docs (may not resolve in Swarm). */
const WG_PUBLIC_URL = (
  process.env.WARPGATE_PUBLIC_URL ||
  process.env.WARPGATE_URL ||
  'https://wg.archgate.com.br'
).replace(/\/$/, '')

/**
 * Virtual host + SNI for Traefik / cookie Domain.
 * Must match the Host() rule (wg.archgate.com.br) — never the container name.
 */
const WG_HOST = (() => {
  const raw = (process.env.WARPGATE_HOST || 'wg.archgate.com.br').trim()
  // Common misconfig: WARPGATE_HOST=archgate-warpgate (breaks CF/Traefik SNI)
  if (raw && !raw.includes('.') && raw !== 'localhost') {
    return 'wg.archgate.com.br'
  }
  return raw || 'wg.archgate.com.br'
})()

/**
 * Optional internal admin URL (e.g. https://archgate-warpgate:8888).
 * Prefer this over public CF hairpin when set.
 */
function parseInternalUrl(): { host: string; port: number } | null {
  const raw = (
    process.env.WARPGATE_INTERNAL_URL ||
    process.env.WARPGATE_URL ||
    ''
  ).trim()
  if (!raw) return null
  try {
    const u = new URL(raw)
    // Only use as dial target if it looks container/internal (not public hostname)
    const host = u.hostname
    if (!host || host === WG_HOST || host.endsWith('.archgate.com.br')) {
      return null
    }
    const port = u.port
      ? Number(u.port)
      : u.protocol === 'http:'
        ? 80
        : 443
    return { host, port }
  } catch {
    return null
  }
}

/**
 * TCP connect target for API calls from console.
 * Prefer explicit CONNECT_HOST, then WARPGATE_URL internal host:port, else public.
 */
const WG_CONNECT_HOST =
  process.env.WARPGATE_CONNECT_HOST ||
  parseInternalUrl()?.host ||
  ''

const WG_CONNECT_PORT = (() => {
  if (process.env.WARPGATE_CONNECT_PORT) {
    return Number(process.env.WARPGATE_CONNECT_PORT)
  }
  const internal = parseInternalUrl()
  if (internal && WG_CONNECT_HOST === internal.host) return internal.port
  if (WG_CONNECT_HOST) return 443
  return 0 // resolve from public URL
})()

const WG_ADMIN_USER = process.env.WARPGATE_ADMIN_USER || 'admin'
const WG_ADMIN_PASSWORD = process.env.WARPGATE_ADMIN_PASSWORD || ''

const AUTH_CACHE_KEY = 'warpgate:admin-cookie'
const AUTH_TTL_MS = 4 * 60 * 1000

export type WarpgateTarget = {
  id: string
  name: string
  description?: string
  options?: Record<string, unknown>
  allow_roles?: unknown[]
}

export type WarpgateRole = {
  id: string
  name: string
  description?: string
}

function configured(): boolean {
  return Boolean(WG_ADMIN_PASSWORD)
}

/** Where we dial TCP (container, Traefik VIP, or public host). */
function connectHostname(): string {
  if (WG_CONNECT_HOST) return WG_CONNECT_HOST
  try {
    return new URL(WG_PUBLIC_URL).hostname
  } catch {
    return WG_HOST
  }
}

function connectPort(): number {
  if (WG_CONNECT_PORT > 0) return WG_CONNECT_PORT
  try {
    const u = new URL(WG_PUBLIC_URL)
    if (u.port) return Number(u.port)
    return u.protocol === 'http:' ? 80 : 443
  } catch {
    return 443
  }
}

type HttpResult = {
  status: number
  headers: https.IncomingHttpHeaders
  body: string
}

/**
 * HTTPS request with explicit SNI (WARPGATE_HOST) independent of TCP peer.
 * Required for Swarm: dial Traefik, present Host/SNI = wg.archgate.com.br.
 */
function warpgateHttps(
  method: string,
  path: string,
  opts?: { headers?: Record<string, string>; body?: string },
): Promise<HttpResult> {
  const body = opts?.body
  const headers: Record<string, string> = {
    Host: WG_HOST,
    Accept: 'application/json',
    ...(opts?.headers || {}),
  }
  if (body !== undefined) {
    headers['Content-Type'] = headers['Content-Type'] || 'application/json'
    headers['Content-Length'] = String(Buffer.byteLength(body))
  }

  // Direct container TLS (self-signed) or lab: allow insecure when explicitly set
  const rejectUnauthorized =
    process.env.NODE_TLS_REJECT_UNAUTHORIZED !== '0' &&
    process.env.WARPGATE_TLS_INSECURE !== '1'

  // When dialing the container directly (not public Host), SNI should match
  // the TCP peer cert subject if possible; still send Host header as WG_HOST
  // for Traefik. For self-signed WG on :8888, SNI = connect host is safer.
  const dialHost = connectHostname()
  const dialPort = connectPort()
  const sni =
    dialHost !== WG_HOST && (dialPort === 8888 || process.env.WARPGATE_SNI_USE_CONNECT === '1')
      ? dialHost
      : WG_HOST

  return new Promise((resolve, reject) => {
    const req = https.request(
      {
        hostname: dialHost,
        port: dialPort,
        method,
        path,
        servername: sni,
        headers,
        rejectUnauthorized,
        timeout: 15_000,
      },
      (res) => {
        const chunks: Buffer[] = []
        res.on('data', (c) => chunks.push(c))
        res.on('end', () => {
          resolve({
            status: res.statusCode || 0,
            headers: res.headers,
            body: Buffer.concat(chunks).toString('utf8'),
          })
        })
      },
    )
    req.on('timeout', () => {
      req.destroy(new Error(`Warpgate timeout ${method} ${path}`))
    })
    req.on('error', reject)
    if (body !== undefined) req.write(body)
    req.end()
  })
}

function parseSetCookie(headers: https.IncomingHttpHeaders): string {
  const raw = headers['set-cookie']
  if (!raw) return ''
  const list = Array.isArray(raw) ? raw : [raw]
  return list
    .map((c) => c.split(';')[0]?.trim())
    .filter(Boolean)
    .join('; ')
}

async function login(): Promise<string> {
  if (!configured()) {
    throw new Error(
      'Warpgate não configurado: defina WARPGATE_ADMIN_PASSWORD no serviço do console',
    )
  }
  const res = await warpgateHttps('POST', '/@warpgate/api/auth/login', {
    body: JSON.stringify({
      username: WG_ADMIN_USER,
      password: WG_ADMIN_PASSWORD,
    }),
  })
  if (res.status < 200 || res.status >= 300) {
    throw new Error(
      `Warpgate login failed: ${res.status} ${res.body.slice(0, 200)}`,
    )
  }
  const cookies = parseSetCookie(res.headers)
  if (!cookies) {
    logger.warn('warpgate login: no Set-Cookie returned')
  }
  return cookies
}

async function sessionCookie(): Promise<string> {
  const entry = await getCachedAuth(AUTH_CACHE_KEY, AUTH_TTL_MS, async () => ({
    value: await login(),
  }))
  return entry.value
}

async function api<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
  retried = false,
): Promise<T> {
  const cookie = await sessionCookie()
  const res = await warpgateHttps(method, path, {
    headers: cookie ? { Cookie: cookie } : {},
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if ((res.status === 401 || res.status === 403) && !retried) {
    invalidateAuthCache(AUTH_CACHE_KEY)
    return api(method, path, body, true)
  }
  if (res.status < 200 || res.status >= 300) {
    throw new Error(
      `Warpgate ${method} ${path}: ${res.status} ${res.body.slice(0, 300)}`,
    )
  }
  if (!res.body) return undefined as T
  try {
    return JSON.parse(res.body) as T
  } catch {
    return res.body as T
  }
}

export async function listWarpgateTargets(): Promise<WarpgateTarget[]> {
  return api<WarpgateTarget[]>('GET', '/@warpgate/admin/api/targets')
}

export async function listWarpgateRoles(): Promise<WarpgateRole[]> {
  return api<WarpgateRole[]>('GET', '/@warpgate/admin/api/roles')
}

export async function ensureRole(
  name: string,
  description?: string,
): Promise<WarpgateRole> {
  const roles = await listWarpgateRoles()
  const found = roles.find((r) => r.name === name)
  if (found) return found
  return api<WarpgateRole>('POST', '/@warpgate/admin/api/roles', {
    name,
    description: description || `ArchGate site role ${name}`,
    is_default: false,
  })
}

export type ApplyTargetInput = {
  name: string
  description?: string
  kind: 'Ssh' | 'MySql' | 'Postgres' | 'Http'
  host: string
  port: number
  username?: string
  password?: string
  database?: string
  roles: string[]
}

export async function upsertTarget(
  input: ApplyTargetInput,
): Promise<{ id: string; name: string }> {
  const targets = await listWarpgateTargets()
  const existing = targets.find((t) => t.name === input.name)

  let options: Record<string, unknown>
  if (input.kind === 'Ssh') {
    options = {
      kind: 'Ssh',
      host: input.host,
      port: input.port,
      username: input.username || 'labuser',
      auth: input.password
        ? { kind: 'Password', password: input.password }
        : { kind: 'PublicKey' },
    }
  } else if (input.kind === 'Postgres') {
    options = {
      kind: 'Postgres',
      host: input.host,
      port: input.port,
      username: input.username || 'postgres',
      password: input.password || '',
      database: input.database || 'postgres',
    }
  } else if (input.kind === 'MySql') {
    options = {
      kind: 'MySql',
      host: input.host,
      port: input.port,
      username: input.username || 'root',
      password: input.password || '',
    }
  } else {
    options = {
      kind: 'Http',
      url: `https://${input.host}:${input.port}`,
    }
  }

  const body = {
    name: input.name,
    description: input.description || `ArchGate site target ${input.name}`,
    options,
  }

  let id: string
  if (existing) {
    await api('PUT', `/@warpgate/admin/api/targets/${existing.id}`, body)
    id = existing.id
  } else {
    const created = await api<{ id: string }>(
      'POST',
      '/@warpgate/admin/api/targets',
      body,
    )
    id = created.id
  }

  for (const roleName of input.roles || []) {
    if (!roleName) continue
    const role = await ensureRole(roleName)
    try {
      await api(
        'POST',
        `/@warpgate/admin/api/targets/${id}/roles/${role.id}`,
        {},
      )
    } catch (e) {
      logger.info(
        { err: String(e), target: input.name, role: roleName },
        'role bind',
      )
    }
  }

  return { id, name: input.name }
}

export async function deleteTarget(id: string): Promise<void> {
  await api('DELETE', `/@warpgate/admin/api/targets/${id}`)
}

export async function createRole(
  name: string,
  description?: string,
): Promise<WarpgateRole> {
  return ensureRole(name, description)
}

export async function deleteRole(id: string): Promise<void> {
  await api('DELETE', `/@warpgate/admin/api/roles/${id}`)
}

/**
 * Best-effort remove of a Warpgate user by username (offboarding).
 * API shape varies by WG version — try list + delete by id.
 */
export async function deleteWarpgateUserByName(
  username: string,
): Promise<{ ok: boolean; detail: string }> {
  try {
    const users = await api<Array<{ id: string; username?: string; name?: string }>>(
      'GET',
      '/@warpgate/admin/api/users',
    )
    const list = Array.isArray(users) ? users : []
    const found = list.find(
      (u) =>
        u.username === username ||
        u.name === username ||
        (u as { credentials?: { username?: string } }).credentials
          ?.username === username,
    )
    if (!found?.id) {
      return { ok: true, detail: 'user not found (already gone)' }
    }
    await api('DELETE', `/@warpgate/admin/api/users/${found.id}`)
    return { ok: true, detail: `deleted user id=${found.id}` }
  } catch (e) {
    // Older builds may not expose users API the same way
    return {
      ok: false,
      detail: (e as Error).message.slice(0, 200),
    }
  }
}

export function warpgateConfigured(): boolean {
  return configured()
}

/** Public browser URL (stock UI). */
export function warpgatePublicUrl(): string {
  return WG_PUBLIC_URL
}

/** Diagnostics for platform module. */
export function warpgateConnectInfo(): {
  public_url: string
  connect_host: string
  connect_port: number
  host_header: string
} {
  return {
    public_url: WG_PUBLIC_URL,
    connect_host: connectHostname(),
    connect_port: connectPort(),
    host_header: WG_HOST,
  }
}
