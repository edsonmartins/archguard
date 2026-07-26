import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '../../../src/server/db'
import {
  checkinCheckout,
  createCheckout,
  expireDueCheckouts,
  getCheckout,
} from '../../../src/server/org-checkouts'

describe('org-checkouts', () => {
  let dir: string

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'org-co-'))
    _resetDbForTests(join(dir, 't.sqlite'))
  })

  afterEach(() => {
    _resetDbForTests()
    rmSync(dir, { recursive: true, force: true })
  })

  it('creates active checkout and checkin', () => {
    const c = createCheckout({
      account_id: 'a1',
      account_slug: 'demo',
      principal: 'alice',
      reason: 'deploy',
      ttl_seconds: 3600,
      status: 'active',
    })
    expect(c.status).toBe('active')
    expect(getCheckout(c.id)?.principal).toBe('alice')
    const closed = checkinCheckout(c.id, 'alice')
    expect(closed?.status).toBe('checked_in')
    expect(closed?.closed_at).toBeTruthy()
  })

  it('expires overdue actives', () => {
    const c = createCheckout({
      account_id: 'a1',
      account_slug: 'demo',
      principal: 'bob',
      reason: 'old',
      ttl_seconds: 1,
      status: 'active',
    })
    // force past expiry
    getDb()
      .prepare(`UPDATE org_checkouts SET expires_at = ? WHERE id = ?`)
      .run('2000-01-01T00:00:00.000Z', c.id)
    expect(expireDueCheckouts()).toBeGreaterThanOrEqual(1)
    expect(getCheckout(c.id)?.status).toBe('expired')
  })
})
