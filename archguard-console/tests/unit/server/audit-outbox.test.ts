import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '@/server/db'
import { recordActivity } from '@/server/activity-log'
import { claimAuditOutbox, markAuditFailed, markAuditPublished } from '@/server/audit-outbox'

describe('audit outbox', () => {
  let dir: string | undefined

  afterEach(() => {
    _resetDbForTests()
    if (dir) rmSync(dir, { recursive: true, force: true })
    dir = undefined
  })

  it('writes activity and redacted outbox event together', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-audit-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    recordActivity('POST', '/archgate/sites/site-a', 'actor-a', 'success', undefined, {
      password: 'must-not-be-in-outbox',
    })
    const row = getDb().prepare('SELECT payload_json FROM audit_outbox').get() as { payload_json: string }
    expect(getDb().prepare('SELECT COUNT(*) AS n FROM activity_log').get()).toMatchObject({ n: 1 })
    expect(row.payload_json).not.toContain('must-not-be-in-outbox')
    const claimed = claimAuditOutbox()
    expect(claimed).toHaveLength(1)
    markAuditFailed(claimed[0].event_id, 'temporary')
    expect(claimAuditOutbox()).toHaveLength(0)
    markAuditPublished(claimed[0].event_id)
  })
})
