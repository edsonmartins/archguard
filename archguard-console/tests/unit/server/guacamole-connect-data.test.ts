import { describe, expect, it } from 'vitest'
import {
  buildGuacConnectData,
  guacClientHashId,
} from '../../../src/server/guacamole-proxy'

describe('buildGuacConnectData', () => {
  it('builds token + GUAC_* query for Client.connect', () => {
    const s = buildGuacConnectData({
      token: 'tok-abc',
      dataSource: 'postgresql',
      connectionId: '42',
    })
    const q = new URLSearchParams(s)
    expect(q.get('token')).toBe('tok-abc')
    expect(q.get('GUAC_DATA_SOURCE')).toBe('postgresql')
    expect(q.get('GUAC_ID')).toBe('42')
    expect(q.get('GUAC_TYPE')).toBe('c')
  })
})

describe('guacClientHashId', () => {
  it('encodes connection id for #/client deep link', () => {
    const h = guacClientHashId('1', 'postgresql')
    expect(h.length).toBeGreaterThan(4)
    // Round-trip: base64url decode
    const pad = h + '==='.slice((h.length + 3) % 4)
    const b64 = pad.replace(/-/g, '+').replace(/_/g, '/')
    const raw = Buffer.from(b64, 'base64').toString('utf8')
    expect(raw.split('\0')).toEqual(['1', 'c', 'postgresql'])
  })
})
