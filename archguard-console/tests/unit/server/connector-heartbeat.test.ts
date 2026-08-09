import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '@/server/db'
import { recordConnectorHeartbeat } from '@/server/connector-heartbeat'

describe('connector heartbeat contract', () => {
  let dir: string | undefined

  afterEach(() => {
    _resetDbForTests()
    if (dir) rmSync(dir, { recursive: true, force: true })
    dir = undefined
  })

  it('persists the latest state idempotently by connector id', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-heartbeat-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    const base = {
      version: 1 as const,
      type: 'heartbeat' as const,
      message_id: 'm-1',
      connector_id: 'connector-lab',
      sent_at: new Date().toISOString(),
      payload: {
        status: 'ready' as const,
        agent_version: '1.0.0',
        capabilities: ['wireguard'],
      },
    }
    recordConnectorHeartbeat(base)
    recordConnectorHeartbeat({
      ...base,
      message_id: 'm-2',
      payload: { ...base.payload, status: 'degraded', capabilities: ['wireguard', 'openvpn'] },
    })
    const row = getDb()
      .prepare('SELECT message_id, status, capabilities_json FROM connector_heartbeats WHERE connector_id = ?')
      .get('connector-lab') as { message_id: string; status: string; capabilities_json: string }
    expect(row.message_id).toBe('m-2')
    expect(row.status).toBe('degraded')
    expect(JSON.parse(row.capabilities_json)).toEqual(['wireguard', 'openvpn'])
  })

  it('rejects unsupported versions and invalid states', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-heartbeat-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    const heartbeat = {
      version: 1,
      type: 'heartbeat' as const,
      message_id: 'm-1',
      connector_id: 'connector-lab',
      sent_at: new Date().toISOString(),
      payload: { status: 'revoked' as const, agent_version: '1.0.0' },
    }
    expect(() => recordConnectorHeartbeat({ ...heartbeat, version: 2 })).toThrow()
    expect(() => recordConnectorHeartbeat({
      ...heartbeat,
      payload: { ...heartbeat.payload, status: 'unknown' as never },
    })).toThrow()
  })
})
