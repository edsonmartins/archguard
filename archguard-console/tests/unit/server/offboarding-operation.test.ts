import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, beginOffboardingOperation, finishOffboardingOperation, getDb, listOffboardingSteps, recordOffboardingStep } from '../../../src/server/db'

describe('offboarding operation lock', () => {
  let dir: string
  beforeEach(() => { dir = mkdtempSync(join(tmpdir(), 'offboarding-op-')); _resetDbForTests(join(dir, 'test.sqlite')) })
  afterEach(() => { _resetDbForTests(); rmSync(dir, { recursive: true, force: true }) })

  it('prevents concurrent operations and allows retry after stale execution', () => {
    const first = beginOffboardingOperation('alice', 'admin')
    expect(() => beginOffboardingOperation('alice', 'admin')).toThrow('já está em execução')
    getDb().prepare('UPDATE offboarding_operations SET updated_at = ? WHERE operation_id = ?').run('2000-01-01T00:00:00.000Z', first)
    const retry = beginOffboardingOperation('alice', 'admin')
    expect(retry).toBe(first)
    finishOffboardingOperation(retry, 'completed')
    expect(getDb().prepare('SELECT status FROM offboarding_operations WHERE operation_id = ?').get(retry)).toEqual({ status: 'completed' })
  })

  it('resumes the latest partial operation and preserves confirmed steps', () => {
    const first = beginOffboardingOperation('alice', 'admin')
    recordOffboardingStep(first, 1, { component: 'console_sessions', ok: true, detail: 'principal blocked locally' })
    finishOffboardingOperation(first, 'partial', 'retry necessário')

    const resumed = beginOffboardingOperation('alice', 'operator')

    expect(resumed).toBe(first)
    expect(listOffboardingSteps(resumed)).toEqual([
      expect.objectContaining({ sequence: 1, component: 'console_sessions', ok: true }),
    ])
    expect(getDb().prepare('SELECT status, started_by FROM offboarding_operations WHERE operation_id = ?').get(resumed)).toEqual({
      status: 'running',
      started_by: 'operator',
    })
  })
})
