// Server-side RustGuac session broker. API keys and target credentials never
// leave the console; the browser receives only a short-lived ws ticket URL.
import { integrationFetch } from './http-integration-client'

const RUSTGUAC_URL = (
  process.env.RUSTGUAC_URL || 'http://archgate-rustguac:8080'
).replace(/\/$/, '')
const RUSTGUAC_PUBLIC_URL = (
  process.env.RUSTGUAC_PUBLIC_URL || RUSTGUAC_URL
).replace(/\/$/, '')
const RUSTGUAC_KEY = process.env.RUSTGUAC_API_KEY || ''

export type RustGuacSession = {
  session_id: string
  client_url?: string
  ws_url?: string
}

export function rustGuacConfigured(): boolean {
  return (
    process.env.RUSTGUAC_ENABLED === '1' &&
    Boolean(RUSTGUAC_URL && RUSTGUAC_KEY)
  )
}

async function api<T>(path: string, body: unknown): Promise<T> {
  const res = await integrationFetch(`${RUSTGUAC_URL}${path}`, {
    method: 'POST',
    integration: 'rustguac',
    headers: {
      Authorization: `Bearer ${RUSTGUAC_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) {
    throw new Error(`RustGuac ${path}: ${res.status} ${text.slice(0, 300)}`)
  }
  try {
    return JSON.parse(text) as T
  } catch {
    throw new Error(`RustGuac ${path}: invalid JSON response`)
  }
}

async function apiGet<T>(path: string): Promise<T> {
  const res = await integrationFetch(`${RUSTGUAC_URL}${path}`, {
    method: 'GET',
    integration: 'rustguac',
    headers: { Authorization: `Bearer ${RUSTGUAC_KEY}` },
  })
  const text = await res.text()
  if (!res.ok) throw new Error(`RustGuac ${path}: ${res.status} ${text.slice(0, 300)}`)
  try {
    return JSON.parse(text) as T
  } catch {
    throw new Error(`RustGuac ${path}: invalid JSON response`)
  }
}

export type RustGuacRecording = {
  name: string
  size_bytes: number
  modified?: string
  created_at?: string
  user?: string
  session_type?: string
  address_book_entry?: string
}

/** List recording metadata server-side; recording bytes never pass through this call. */
export async function listRustGuacRecordings(): Promise<RustGuacRecording[]> {
  if (!rustGuacConfigured()) throw new Error('RustGuac não configurado')
  return apiGet<RustGuacRecording[]>('/api/recordings')
}

function sessionType(protocol: string): 'ssh' | 'rdp' | 'vnc' {
  const p = protocol.toLowerCase()
  return p === 'rdp' || p === 'vnc' ? p : 'ssh'
}

/** Build browser-safe URLs from RustGuac's server response. */
export function buildRustGuacUrls(
  created: Pick<RustGuacSession, 'session_id' | 'client_url' | 'ws_url'>,
  ticket: string,
  publicBase = RUSTGUAC_PUBLIC_URL,
) {
  if (!created.session_id) throw new Error('RustGuac retornou sessão sem id')
  if (!ticket) throw new Error('RustGuac retornou ticket vazio')
  const client = created.client_url || `/client/${created.session_id}`
  const ws = created.ws_url || `/ws/${created.session_id}`
  const base = publicBase.replace(/\/$/, '')
  return {
    embed_url: `${client.startsWith('http') ? client : `${base}${client}`}?ticket=${encodeURIComponent(ticket)}`,
    tunnel_url: `${ws.startsWith('ws') ? ws : base.replace(/^http/, 'ws') + ws}`,
    connect_data: '',
    // RustGuac tickets are currently valid for 30 seconds.
    expires_in: 30,
  }
}

export async function issueRustGuacSession(input: {
  protocol: string
  hostname: string
  port: number
  username?: string
  password?: string
  private_key?: string
  session_policy?: {
    enable_drive?: boolean
    enable_recording?: boolean
    disable_copy?: boolean
    disable_paste?: boolean
  }
}): Promise<{ session_id: string; embed_url: string; tunnel_url: string; connect_data: string; expires_in: number }> {
  if (!rustGuacConfigured()) throw new Error('RustGuac não configurado')
  const created = await api<RustGuacSession>('/api/sessions', {
    session_type: sessionType(input.protocol),
    hostname: input.hostname,
    port: input.port,
    ...(input.username ? { username: input.username } : {}),
    ...(input.password ? { password: input.password } : {}),
    ...(input.private_key ? { private_key: input.private_key } : {}),
    ...(input.session_policy || {}),
  })
  const result = await api<{ ticket?: string }>('/api/ws-ticket', {})
  return { session_id: created.session_id, ...buildRustGuacUrls(created, result.ticket || '') }
}

/** Close the broker session; the API key remains server-side. */
export async function closeRustGuacSession(sessionId: string): Promise<void> {
  if (!rustGuacConfigured()) throw new Error('RustGuac não configurado')
  const res = await integrationFetch(`${RUSTGUAC_URL}/api/sessions/${encodeURIComponent(sessionId)}`, {
    method: 'DELETE',
    integration: 'rustguac',
    headers: { Authorization: `Bearer ${RUSTGUAC_KEY}` },
  })
  if (!res.ok && res.status !== 404) throw new Error(`RustGuac close: ${res.status}`)
}
