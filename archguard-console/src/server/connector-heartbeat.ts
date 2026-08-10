import { getDb } from './db'

export type ConnectorHeartbeat = {
  version: number
  type: 'heartbeat'
  message_id: string
  connector_id: string
  sent_at: string
  payload: {
    status: 'ready' | 'degraded' | 'revoked'
    agent_version: string
    capabilities?: string[]
    uptime_seconds?: number
    last_config_revision?: string
    inventory?: {
      os?: string
      os_release?: string
      architecture?: string
      hostname?: string
      interfaces?: string[]
    }
  }
}

const ID_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$/

export function recordConnectorHeartbeat(input: ConnectorHeartbeat): void {
  if (input.version !== 1 || input.type !== 'heartbeat') throw new Error('unsupported heartbeat envelope')
  if (!ID_RE.test(input.connector_id) || !input.message_id || input.message_id.length > 128) {
    throw new Error('invalid connector heartbeat identity')
  }
  const p = input.payload
  if (!p || !['ready', 'degraded', 'revoked'].includes(p.status) || !p.agent_version) {
    throw new Error('invalid connector heartbeat payload')
  }
  const now = new Date().toISOString()
  getDb().prepare(
    `INSERT INTO connector_heartbeats
       (connector_id, message_id, status, agent_version, capabilities_json, last_seen_at, payload_json)
     VALUES (?, ?, ?, ?, ?, ?, ?)
     ON CONFLICT(connector_id) DO UPDATE SET
       message_id=excluded.message_id,
       status=excluded.status,
       agent_version=excluded.agent_version,
       capabilities_json=excluded.capabilities_json,
       last_seen_at=excluded.last_seen_at,
       payload_json=excluded.payload_json`,
  ).run(
    input.connector_id,
    input.message_id,
    p.status,
    p.agent_version,
    JSON.stringify(p.capabilities || []),
    now,
    JSON.stringify(p),
  )
}
