import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  _resetDbForTests,
  createAccessGrant,
  getLatestAccessGrant,
  hasActiveAccessGrant,
  getLegacyAccessGrantStatus,
  migrateLegacyAccessGrants,
  listAccessGrantsForPrincipal,
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

  it('paginates grant inventory without crossing principals', () => {
    for (let i = 0; i < 3; i += 1) {
      createAccessGrant({ grant_id: `g-${i}`, principal: 'alice', target: `db-${i}`, expires_at: '2099-01-01T00:00:00.000Z' })
    }
    createAccessGrant({ grant_id: 'g-bob', principal: 'bob', target: 'db-bob', expires_at: '2099-01-01T00:00:00.000Z' })

    expect(listAccessGrantsForPrincipal('alice', 2, 0)).toMatchObject({ total: 3, has_more: true, items: expect.any(Array) })
    expect(listAccessGrantsForPrincipal('alice', 2, 2)).toMatchObject({ total: 3, has_more: false, items: [expect.objectContaining({ principal: 'alice' })] })
  })

  it('persists canonical identity id and matches grants during migration', () => {
    createAccessGrant({ grant_id: 'g-canonical', principal: 'alice', identity_id: 'identity-1', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    expect(getLatestAccessGrant('alice', 'db')).toMatchObject({ identity_id: 'identity-1' })
    expect(hasActiveAccessGrant('alice', 'db', 'identity-1', Date.parse('2026-01-01T00:00:00.000Z'))).toBe(true)
    expect(hasActiveAccessGrant('alice', 'db', 'other-identity', Date.parse('2026-01-01T00:00:00.000Z'))).toBe(false)
  })

  it('reports legacy grants without inventing identity mappings', () => {
    createAccessGrant({ grant_id: 'g-legacy-a', principal: 'alice', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-legacy-b', principal: 'bob', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-canonical', principal: 'carol', identity_id: 'identity-3', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    expect(getLegacyAccessGrantStatus()).toMatchObject({ total: 2, principals: 2 })
  })

  it('migrates only legacy grants for an explicitly mapped principal', () => {
    createAccessGrant({ grant_id: 'g-legacy-a', principal: 'alice', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-canonical-a', principal: 'alice', identity_id: 'old-id', target: 'db-2', expires_at: '2099-01-01T00:00:00.000Z' })
    createAccessGrant({ grant_id: 'g-legacy-b', principal: 'bob', target: 'db', expires_at: '2099-01-01T00:00:00.000Z' })
    expect(migrateLegacyAccessGrants(' alice ', ' identity-1 ')).toMatchObject({ principal: 'alice', identity_id: 'identity-1', affected: 1 })
    expect(getLatestAccessGrant('alice', 'db')?.identity_id).toBe('identity-1')
    expect(getLatestAccessGrant('alice', 'db-2')?.identity_id).toBe('old-id')
    expect(getLatestAccessGrant('bob', 'db')?.identity_id).toBeNull()
  })
})
