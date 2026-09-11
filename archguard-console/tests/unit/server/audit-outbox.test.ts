import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '@/server/db'
import { recordActivity } from '@/server/activity-log'
import { claimAuditOutbox, getAuditOutboxStatus, markAuditFailed, markAuditPublished, recoverStaleAuditClaims } from '@/server/audit-outbox'

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

  it('requeues a publishing claim left by a crashed forwarder', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-audit-recovery-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    recordActivity('POST', '/archgate/sites/site-a', 'actor-a', 'success')
    const claimed = claimAuditOutbox()
    expect(claimed).toHaveLength(1)
    getDb().prepare("UPDATE audit_outbox SET claimed_at = ?, available_at = ? WHERE event_id = ?")
      .run(new Date(Date.now() - 120_000).toISOString(), new Date(Date.now() - 120_000).toISOString(), claimed[0].event_id)
    expect(recoverStaleAuditClaims()).toBe(1)
    const row = getDb().prepare('SELECT status, available_at, claimed_at, last_error FROM audit_outbox').get() as Record<string, unknown>
    expect(row.status).toBe('failed')
    expect(row.claimed_at).toBeNull()
    expect(row.last_error).toContain('claim expired')
    expect(claimAuditOutbox()).toHaveLength(1)
  })

  it('does not reclaim a fresh publishing claim', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-audit-fresh-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    recordActivity('POST', '/archgate/sites/site-a', 'actor-a', 'success')
    expect(claimAuditOutbox()).toHaveLength(1)
    expect(recoverStaleAuditClaims()).toBe(0)
    expect(claimAuditOutbox()).toHaveLength(0)
  })

  it('reports blocked health and stale claims without exposing payloads', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-audit-status-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    recordActivity('POST', '/archgate/sites/site-a', 'actor-a', 'success')
    const claimed = claimAuditOutbox()
    getDb().prepare('UPDATE audit_outbox SET claimed_at = ? WHERE event_id = ?')
      .run(new Date(Date.now() - 120_000).toISOString(), claimed[0].event_id)
    const status = getAuditOutboxStatus()
    expect(status.health).toBe('blocked')
    expect(status.stale_claims).toBe(1)
    expect(status).not.toHaveProperty('payload_json')
  })
})
