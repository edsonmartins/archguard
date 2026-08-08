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

function sessionType(protocol: string): 'ssh' | 'rdp' | 'vnc' {
  const p = protocol.toLowerCase()
  return p === 'rdp' || p === 'vnc' ? p : 'ssh'
}

export async function issueRustGuacSession(input: {
  protocol: string
  hostname: string
  port: number
  username?: string
  password?: string
  private_key?: string
}): Promise<{ embed_url: string; tunnel_url: string; connect_data: string; expires_in: number }> {
  if (!rustGuacConfigured()) throw new Error('RustGuac não configurado')
  const created = await api<RustGuacSession>('/api/sessions', {
    session_type: sessionType(input.protocol),
    hostname: input.hostname,
    port: input.port,
    ...(input.username ? { username: input.username } : {}),
    ...(input.password ? { password: input.password } : {}),
    ...(input.private_key ? { private_key: input.private_key } : {}),
  })
  if (!created.session_id) throw new Error('RustGuac retornou sessão sem id')
  const result = await api<{ ticket?: string }>('/api/ws-ticket', {})
  if (!result.ticket) throw new Error('RustGuac retornou ticket vazio')
  const client = created.client_url || `/client/${created.session_id}`
  const ws = created.ws_url || `/ws/${created.session_id}`
  // The API may be private while the ticket URL must be reachable by the browser.
  const base = RUSTGUAC_PUBLIC_URL
  return {
    embed_url: `${client.startsWith('http') ? client : `${base}${client}`}?ticket=${encodeURIComponent(result.ticket)}`,
    tunnel_url: `${ws.startsWith('ws') ? ws : base.replace(/^http/, 'ws') + ws}`,
    connect_data: '',
    expires_in: 30,
  }
}
