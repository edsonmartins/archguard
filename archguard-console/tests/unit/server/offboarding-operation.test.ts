import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, beginOffboardingOperation, finishOffboardingOperation, getDb } from '../../../src/server/db'

describe('offboarding operation lock', () => {
  let dir: string
  beforeEach(() => { dir = mkdtempSync(join(tmpdir(), 'offboarding-op-')); _resetDbForTests(join(dir, 'test.sqlite')) })
  afterEach(() => { _resetDbForTests(); rmSync(dir, { recursive: true, force: true }) })

  it('prevents concurrent operations and allows retry after stale execution', () => {
    const first = beginOffboardingOperation('alice', 'admin')
    expect(() => beginOffboardingOperation('alice', 'admin')).toThrow('já está em execução')
    getDb().prepare('UPDATE offboarding_operations SET updated_at = ? WHERE operation_id = ?').run('2000-01-01T00:00:00.000Z', first)
    const retry = beginOffboardingOperation('alice', 'admin')
    expect(retry).not.toBe(first)
    finishOffboardingOperation(retry, 'completed')
    expect(getDb().prepare('SELECT status FROM offboarding_operations WHERE operation_id = ?').get(retry)).toEqual({ status: 'completed' })
  })
})
