import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '../../../src/server/db'
import {
  approveCheckout,
  checkinCheckout,
  createCheckout,
  denyCheckout,
  expireDueCheckouts,
  forceCloseCheckoutsForPrincipal,
  getCheckout,
  listPendingCheckouts,
  samePrincipal,
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

  it('dual-control: approve by other, reject self-approve', () => {
    const c = createCheckout({
      account_id: 'a1',
      account_slug: 'apple',
      principal: 'alice@x',
      reason: 'ship ios',
      ttl_seconds: 3600,
      status: 'pending',
    })
    expect(listPendingCheckouts()).toHaveLength(1)
    expect(samePrincipal('alice@x', 'Alice@X')).toBe(true)
    expect(() => approveCheckout(c.id, 'alice@x')).toThrow(/Self-approve/)
    const ok = approveCheckout(c.id, 'bob@x')
    expect(ok?.status).toBe('active')
    expect(ok?.approved_by).toBe('bob@x')
    expect(listPendingCheckouts()).toHaveLength(0)
  })

  it('deny by other principal', () => {
    const c = createCheckout({
      account_id: 'a1',
      account_slug: 'apple',
      principal: 'alice',
      reason: 'nope',
      ttl_seconds: 600,
      status: 'pending',
    })
    const d = denyCheckout(c.id, 'bob')
    expect(d?.status).toBe('denied')
    expect(d?.approved_by).toBe('bob')
  })

  it('force-closes active and pending checkouts for principal (A1 offboarding)', () => {
    const active = createCheckout({
      account_id: 'a1',
      account_slug: 'vendax',
      principal: 'edson@integrall.tech',
      reason: 'support',
      ttl_seconds: 3600,
      status: 'active',
    })
    const pending = createCheckout({
      account_id: 'a2',
      account_slug: 'apple',
      principal: 'edson@integrall.tech',
      reason: 'ship',
      ttl_seconds: 3600,
      status: 'pending',
    })
    const other = createCheckout({
      account_id: 'a3',
      account_slug: 'gcp',
      principal: 'other@integrall.tech',
      reason: 'ops',
      ttl_seconds: 3600,
      status: 'active',
    })
    // bare username match against email principal
    expect(forceCloseCheckoutsForPrincipal('edson')).toBe(2)
    expect(getCheckout(active.id)?.status).toBe('checked_in')
    expect(getCheckout(pending.id)?.status).toBe('checked_in')
    expect(getCheckout(other.id)?.status).toBe('active')
    expect(forceCloseCheckoutsForPrincipal('edson')).toBe(0)
  })
})
