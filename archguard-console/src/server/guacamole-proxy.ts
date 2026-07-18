// Server-side Guacamole REST (admin). Passwords never returned to browser.
// Token is cached briefly to avoid login-per-request storms.

import {
  getCachedAuth,
  integrationFetch,
  invalidateAuthCache,
} from './http-integration-client'

const GUAC_URL = (
  process.env.GUACAMOLE_URL || 'http://archgate-gw_guacamole:8080'
).replace(/\/$/, '')
const GUAC_USER = process.env.GUACAMOLE_ADMIN_USER || 'guacadmin'
const GUAC_PASS =
  process.env.GUACAMOLE_ADMIN_PASSWORD || process.env.GUACADMIN_PASSWORD || ''

const AUTH_CACHE_KEY = 'guacamole:admin-token'
const AUTH_TTL_MS = 4 * 60 * 1000

export type GuacConnectionSummary = {
  id: string
  name: string
  protocol: string
  parentIdentifier?: string
  activeConnections?: number
}

function configured(): boolean {
  // Require explicit password — never default to "guacadmin" in prod paths.
  return Boolean(GUAC_URL && GUAC_USER && GUAC_PASS)
}

async function login(): Promise<{ token: string; dataSource: string }> {
  if (!GUAC_PASS) {
    throw new Error(
      'Guacamole não configurado: defina GUACAMOLE_ADMIN_PASSWORD no serviço do console',
    )
  }
  const body = new URLSearchParams({
    username: GUAC_USER,
    password: GUAC_PASS,
  })
  const res = await integrationFetch(`${GUAC_URL}/api/tokens`, {
    method: 'POST',
    integration: 'guacamole',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body,
  })
  if (!res.ok) {
    const t = await res.text()
    throw new Error(`Guacamole login failed: ${res.status} ${t.slice(0, 200)}`)
  }
  const data = (await res.json()) as {
    authToken: string
    dataSource?: string
  }
  return {
    token: data.authToken,
    dataSource: data.dataSource || 'postgresql',
  }
}

async function sessionAuth(): Promise<{ token: string; dataSource: string }> {
  const entry = await getCachedAuth(AUTH_CACHE_KEY, AUTH_TTL_MS, async () => {
    const s = await login()
    return { value: s.token, meta: { dataSource: s.dataSource } }
  })
  return {
    token: entry.value,
    dataSource: entry.meta?.dataSource || 'postgresql',
  }
}

async function api<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
  retried = false,
): Promise<T> {
  const { token, dataSource } = await sessionAuth()
  const url = path.includes('?')
    ? `${GUAC_URL}${path}&token=${encodeURIComponent(token)}`
    : `${GUAC_URL}${path}?token=${encodeURIComponent(token)}`
  const res = await integrationFetch(url.replace('{ds}', dataSource), {
    method,
    integration: 'guacamole',
    headers: {
      'Guacamole-Token': token,
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if ((res.status === 401 || res.status === 403) && !retried) {
    invalidateAuthCache(AUTH_CACHE_KEY)
    return api(method, path, body, true)
  }
  const text = await res.text()
  if (!res.ok) {
    throw new Error(
      `Guacamole ${method} ${path}: ${res.status} ${text.slice(0, 300)}`,
    )
  }
  if (!text) return undefined as T
  try {
    return JSON.parse(text) as T
  } catch {
    return text as T
  }
}

export function guacamoleConfigured(): boolean {
  return configured()
}

export async function listConnections(): Promise<GuacConnectionSummary[]> {
  const raw = await api<
    Record<
      string,
      {
        name?: string
        protocol?: string
        parentIdentifier?: string
        activeConnections?: number
      }
    >
  >('GET', '/api/session/data/{ds}/connections')
  if (!raw || typeof raw !== 'object') return []
  return Object.entries(raw).map(([id, v]) => ({
    id,
    name: v?.name || id,
    protocol: v?.protocol || 'unknown',
    parentIdentifier: v?.parentIdentifier,
    activeConnections: v?.activeConnections,
  }))
}

export type CreateGuacConnectionInput = {
  name: string
  protocol: 'ssh' | 'rdp' | 'vnc'
  hostname: string
  port: number
  username?: string
  password?: string
}

export async function createConnection(
  input: CreateGuacConnectionInput,
): Promise<{ id: string; name: string }> {
  const parameters: Record<string, string> = {
    hostname: input.hostname,
    port: String(input.port),
  }
  if (input.username) parameters.username = input.username
  if (input.password) parameters.password = input.password
  if (input.protocol === 'ssh' || input.protocol === 'rdp') {
    parameters['recording-path'] = '/recordings'
    parameters['recording-name'] =
      '${GUAC_DATE}-${GUAC_TIME}-${GUAC_USERNAME}-' + input.name
    parameters['create-recording-path'] = 'true'
  }

  const body = {
    parentIdentifier: 'ROOT',
    name: input.name,
    protocol: input.protocol,
    parameters,
    attributes: {},
  }
  const created = await api<{ identifier?: string }>(
    'POST',
    '/api/session/data/{ds}/connections',
    body,
  )
  const id = created?.identifier || ''
  if (!id) {
    const all = await listConnections()
    const found = all.find((c) => c.name === input.name)
    return { id: found?.id || '', name: input.name }
  }
  return { id, name: input.name }
}

export async function deleteConnection(id: string): Promise<void> {
  await api('DELETE', `/api/session/data/{ds}/connections/${id}`)
}
