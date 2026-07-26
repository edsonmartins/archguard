import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '../../../src/server/db'
import {
  deleteOrgAccount,
  getOrgAccount,
  listOrgAccounts,
  seedDefaultOrgAccountsIfEmpty,
  upsertOrgAccount,
} from '../../../src/server/org-accounts'

describe('org-accounts persistence', () => {
  let dir: string

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'org-acc-'))
    _resetDbForTests(join(dir, 't.sqlite'))
  })

  afterEach(() => {
    _resetDbForTests()
    rmSync(dir, { recursive: true, force: true })
  })

  it('seeds defaults once and never stores password fields', () => {
    const n = seedDefaultOrgAccountsIfEmpty('test')
    expect(n).toBeGreaterThan(5)
    expect(seedDefaultOrgAccountsIfEmpty('test')).toBe(0)
    const all = listOrgAccounts()
    expect(all.some((a) => a.slug === 'apple-appstore-integrall')).toBe(true)
    expect(all.every((a) => a.criticality === 'P0' ? a.requires_dual_control : true || true)).toBe(
      true,
    )
    const apple = getOrgAccount('apple-appstore-integrall')!
    expect(apple.requires_dual_control).toBe(true)
    expect(JSON.stringify(apple)).not.toMatch(/password_value|secret_value/)
    expect(apple.secret_ref).toContain('secret/data/org/')
  })

  it('upserts and deletes by slug', () => {
    upsertOrgAccount(
      {
        slug: 'demo-saas',
        name: 'Demo SaaS',
        category: 'vendor',
        criticality: 'P2',
      },
      'admin@test',
    )
    const got = getOrgAccount('demo-saas')
    expect(got?.name).toBe('Demo SaaS')
    expect(deleteOrgAccount('demo-saas')).toBe(true)
    expect(getOrgAccount('demo-saas')).toBeNull()
  })
})
