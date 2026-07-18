// Server-side OpenBao API (token never exposed to browser).
// Prefer least-privilege APP_TOKEN; root token only in non-prod or explicit allow.

import { logger } from './logger'
import { integrationFetch } from './http-integration-client'

const BAO_ADDR = (
  process.env.OPENBAO_ADDR ||
  process.env.OPENBAO_URL ||
  'http://archgate-secrets_openbao:8200'
).replace(/\/$/, '')

const BAO_UNSEAL_KEY = process.env.OPENBAO_UNSEAL_KEY || ''

/**
 * Resolve API token with least-privilege first.
 * OPENBAO_ROOT_TOKEN is rejected in production unless ALLOW_OPENBAO_ROOT_TOKEN=1.
 */
function resolveBaoToken(): { token: string; kind: 'app' | 'root' | 'none' } {
  const app =
    process.env.OPENBAO_APP_TOKEN ||
    process.env.OPENBAO_TOKEN ||
    ''
  if (app) return { token: app, kind: 'app' }

  const root = process.env.OPENBAO_ROOT_TOKEN || ''
  if (!root) return { token: '', kind: 'none' }

  const isProd = process.env.NODE_ENV === 'production'
  const allowRoot =
    process.env.ALLOW_OPENBAO_ROOT_TOKEN === '1' ||
    process.env.ARCHGATE_LAB === '1'

  if (isProd && !allowRoot) {
    logger.error(
      'OPENBAO_ROOT_TOKEN set but blocked in production — use OPENBAO_APP_TOKEN (or ALLOW_OPENBAO_ROOT_TOKEN=1 for emergency lab)',
    )
    return { token: '', kind: 'none' }
  }

  logger.warn(
    'using OPENBAO_ROOT_TOKEN — configure OPENBAO_APP_TOKEN with least privilege for production',
  )
  return { token: root, kind: 'root' }
}

const resolved = resolveBaoToken()
const BAO_TOKEN = resolved.token

export type OpenBaoHealth = {
  initialized?: boolean
  sealed?: boolean
  standby?: boolean
  version?: string
  cluster_name?: string
  cluster_id?: string
  server_time_utc?: number
}

export type OpenBaoMount = {
  path: string
  type: string
  description?: string
  accessor?: string
}

export type OpenBaoAuthMethod = {
  path: string
  type: string
  description?: string
}

function configured(): boolean {
  return Boolean(BAO_ADDR)
}

function tokenConfigured(): boolean {
  return Boolean(BAO_TOKEN)
}

async function api<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
  opts?: { token?: boolean },
): Promise<{ status: number; data: T }> {
  const useToken = opts?.token !== false
  const res = await integrationFetch(`${BAO_ADDR}/v1${path}`, {
    method,
    integration: 'openbao',
    headers: {
      'Content-Type': 'application/json',
      ...(useToken && BAO_TOKEN ? { 'X-Vault-Token': BAO_TOKEN } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  let data: T
  try {
    data = text ? (JSON.parse(text) as T) : ({} as T)
  } catch {
    data = { raw: text } as T
  }
  return { status: res.status, data }
}

export function openbaoConfigured(): boolean {
  return configured()
}

export function openbaoTokenConfigured(): boolean {
  return tokenConfigured()
}

export function openbaoTokenKind(): 'app' | 'root' | 'none' {
  return resolved.kind
}

export async function getHealth(): Promise<
  OpenBaoHealth & { http_status: number }
> {
  const { status, data } = await api<OpenBaoHealth>(
    'GET',
    '/sys/health',
    undefined,
    {
      token: false,
    },
  )
  return { ...data, http_status: status }
}

export async function getSealStatus(): Promise<{
  sealed?: boolean
  t?: number
  n?: number
  progress?: number
  type?: string
  initialized?: boolean
  version?: string
}> {
  const { data } = await api<Record<string, unknown>>(
    'GET',
    '/sys/seal-status',
    undefined,
    {
      token: false,
    },
  )
  return data as {
    sealed?: boolean
    t?: number
    n?: number
    progress?: number
    type?: string
    initialized?: boolean
    version?: string
  }
}

export async function unsealWithEnvKey(): Promise<{
  sealed: boolean
  progress?: number
}> {
  if (!BAO_UNSEAL_KEY) {
    throw new Error('OPENBAO_UNSEAL_KEY não configurada no serviço do console')
  }
  const { status, data } = await api<{ sealed?: boolean; progress?: number }>(
    'PUT',
    '/sys/unseal',
    { key: BAO_UNSEAL_KEY },
    { token: false },
  )
  if (status >= 400) {
    throw new Error(
      `unseal failed HTTP ${status}: ${JSON.stringify(data).slice(0, 200)}`,
    )
  }
  return { sealed: !!data.sealed, progress: data.progress }
}

export async function listMounts(): Promise<OpenBaoMount[]> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN (ou TOKEN) ausente no console')
  }
  const { status, data } = await api<{
    data?: Record<
      string,
      { type?: string; description?: string; accessor?: string }
    >
  }>('GET', '/sys/mounts')
  if (status >= 400) throw new Error(`mounts HTTP ${status}`)
  const map =
    data.data || (data as unknown as Record<string, { type?: string }>)
  return Object.entries(map)
    .filter(([k]) => !k.startsWith('request_'))
    .map(([path, v]) => ({
      path,
      type: (v as { type?: string })?.type || 'unknown',
      description: (v as { description?: string })?.description,
      accessor: (v as { accessor?: string })?.accessor,
    }))
    .sort((a, b) => a.path.localeCompare(b.path))
}

export async function listAuthMethods(): Promise<OpenBaoAuthMethod[]> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN ausente')
  }
  const { status, data } = await api<{
    data?: Record<string, { type?: string; description?: string }>
  }>('GET', '/sys/auth')
  if (status >= 400) throw new Error(`auth HTTP ${status}`)
  const map = data.data || {}
  return Object.entries(map).map(([path, v]) => ({
    path,
    type: v?.type || 'unknown',
    description: v?.description,
  }))
}

export async function listPolicies(): Promise<string[]> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN ausente')
  }
  const { status, data } = await api<{ data?: { keys?: string[] } }>(
    'LIST',
    '/sys/policies/acl',
  )
  if (status >= 400) {
    const r2 = await api<{ data?: { keys?: string[] } }>(
      'GET',
      '/sys/policies/acl?list=true',
    )
    return r2.data?.data?.keys || []
  }
  return data.data?.keys || []
}

/** List lease IDs under database/creds/<role> (lab: lab-readonly). */
export async function listDbLeases(
  role = 'lab-readonly',
): Promise<{ role: string; lease_ids: string[] }> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN ausente')
  }
  const path = `/sys/leases/lookup/database/creds/${role}`
  const { status, data } = await api<{ data?: { keys?: string[] } }>(
    'LIST',
    path,
  )
  if (status >= 400) {
    return { role, lease_ids: [] }
  }
  const keys = data.data?.keys || []
  const lease_ids = keys.map((k) =>
    k.includes('/') ? k : `database/creds/${role}/${k}`,
  )
  return { role, lease_ids }
}

export async function revokeLease(lease_id: string): Promise<void> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN ausente')
  }
  const { status, data } = await api('PUT', '/sys/leases/revoke', {
    lease_id,
  })
  if (status >= 400) {
    throw new Error(`revoke failed: ${JSON.stringify(data).slice(0, 200)}`)
  }
}

export async function getJwtConfig(): Promise<Record<string, unknown> | null> {
  if (!tokenConfigured()) return null
  const { status, data } = await api<{ data?: Record<string, unknown> }>(
    'GET',
    '/auth/jwt/config',
  )
  if (status >= 400) return null
  const d = { ...(data.data || {}) }
  if (Array.isArray(d.jwt_validation_pubkeys)) {
    d.jwt_validation_pubkeys = (d.jwt_validation_pubkeys as string[]).map(
      (p) =>
        typeof p === 'string' && p.length > 40 ? `${p.slice(0, 40)}…` : p,
    )
  }
  return d
}

export function openbaoAddr(): string {
  return BAO_ADDR
}

/**
 * Read a secret string from OpenBao KV (v2 preferred, v1 fallback).
 * Path forms accepted:
 * - `secret/data/archgate/targets/foo` (KV v2 full path)
 * - `secret/archgate/targets/foo` (auto-inserts /data/)
 * - `kv/data/...`
 *
 * Returns password/value from data.password | data.value | data.secret | first string field.
 */
export async function readSecretValue(
  secretRef: string,
): Promise<string | undefined> {
  if (!tokenConfigured()) {
    throw new Error('OPENBAO_APP_TOKEN ausente para resolver secret_ref')
  }
  const ref = secretRef.replace(/^\//, '').replace(/\/$/, '')
  if (!ref) return undefined

  const candidates = kvPathsToTry(ref)
  for (const path of candidates) {
    const { status, data } = await api<{
      data?: {
        data?: Record<string, unknown>
        [k: string]: unknown
      }
    }>('GET', `/${path}`)
    if (status >= 400) continue
    const payload = data?.data
    // KV v2: { data: { data: { password: "..." }, metadata: ... } }
    const inner =
      payload &&
      typeof payload === 'object' &&
      payload.data &&
      typeof payload.data === 'object'
        ? (payload.data as Record<string, unknown>)
        : (payload as Record<string, unknown> | undefined)
    if (!inner) continue
    const value = pickSecretField(inner)
    if (value) return value
  }
  return undefined
}

function kvPathsToTry(ref: string): string[] {
  const paths = [ref]
  // secret/foo → secret/data/foo
  const m = ref.match(/^([^/]+)\/(?!data\/)(.+)$/)
  if (m && m[1] !== 'sys' && m[1] !== 'auth') {
    paths.push(`${m[1]}/data/${m[2]}`)
  }
  // already has data/
  if (ref.includes('/data/')) {
    paths.push(ref.replace('/data/', '/'))
  }
  return [...new Set(paths)]
}

function pickSecretField(data: Record<string, unknown>): string | undefined {
  for (const k of ['password', 'value', 'secret', 'pass', 'token', 'key']) {
    const v = data[k]
    if (typeof v === 'string' && v.length > 0) return v
  }
  for (const v of Object.values(data)) {
    if (typeof v === 'string' && v.length > 0) return v
  }
  return undefined
}
