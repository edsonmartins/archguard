import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  _resetDbForTests,
  createAccessGrant,
  getLatestAccessGrant,
  hasActiveAccessGrant,
  revokeAccessGrantsForPrincipal,
} from '../../../src/server/db'
import { grantTtlSeconds } from '../../../src/server/lifecycle-fn'

describe('console access grant expiry', () => {
  let dir: string

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'access-grants-'))
    _resetDbForTests(join(dir, 'test.sqlite'))
  })

  afterEach(() => {
    _resetDbForTests()
    rmSync(dir, { recursive: true, force: true })
  })

  it('parses bounded TTL values and rejects invalid or excessive values', () => {
    expect(grantTtlSeconds()).toBe(8 * 3600)
    expect(grantTtlSeconds('30m')).toBe(1800)
    expect(grantTtlSeconds('1d')).toBe(86400)
    expect(() => grantTtlSeconds('30s')).toThrow('intervalo')
    expect(() => grantTtlSeconds('25h')).toThrow('intervalo')
    expect(() => grantTtlSeconds('forever')).toThrow('TTL inválido')
  })

  it('keeps the newest grant and revokes all grants during offboarding', () => {
    createAccessGrant({ grant_id: 'g-old', principal: 'alice', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-new', principal: 'alice', target: 'db', expires_at: '2000-01-01T00:00:00.000Z' })
    const latest = getLatestAccessGrant('alice', 'db')
    expect(latest?.grant_id).toBe('g-new')
    expect(revokeAccessGrantsForPrincipal('alice')).toBe(2)
    expect(getLatestAccessGrant('alice', 'db')?.revoked_at).toBeTruthy()
  })

  it('keeps an overlapping grant active when another one expires', () => {
    createAccessGrant({ grant_id: 'g-expired', principal: 'alice', target: 'db', expires_at: '2000-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-valid', principal: 'alice', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    expect(hasActiveAccessGrant('alice', 'db', Date.parse('2026-01-01T00:00:00.000Z'))).toBe(true)
  })
})
