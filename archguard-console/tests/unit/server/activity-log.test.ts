import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '@/server/db'
import { recordActivity, queryActivityLog } from '@/server/activity-log'

let tempDir: string

beforeEach(() => {
  tempDir = mkdtempSync(join(tmpdir(), 'archguard-test-'))
  _resetDbForTests(join(tempDir, 'test.sqlite'))
})

afterEach(() => {
  _resetDbForTests('')
  rmSync(tempDir, { recursive: true, force: true })
})

describe('recordActivity (persisted)', () => {
  it('persists a successful mutation and reads it back', () => {
    recordActivity('POST', '/v1/person', 'alice', 'success')
    const rows = queryActivityLog()
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      method: 'POST',
      path: '/v1/person',
      actor: 'alice',
      result: 'success',
      action: 'Criar person',
    })
  })

  it('records error mutations with the error message', () => {
    recordActivity(
      'DELETE',
      '/v1/person/bob',
      'admin',
      'error',
      'Kanidm API 404',
    )
    const [row] = queryActivityLog()
    expect(row.result).toBe('error')
    expect(row.errorMessage).toBe('Kanidm API 404')
    expect(row.target).toBe('bob')
    expect(row.action).toBe('Excluir person')
  })

  it('survives reconnects (the row is on disk, not in memory)', () => {
    recordActivity('POST', '/v1/group', 'alice', 'success')
    // Simulate process restart by re-opening the DB.
    const path = process.env.ARCHGUARD_DB_PATH!
    _resetDbForTests(path)
    expect(queryActivityLog()).toHaveLength(1)
  })

  it('returns rows ordered by timestamp DESC (newest first)', () => {
    recordActivity('POST', '/v1/group', 'a', 'success')
    recordActivity('POST', '/v1/oauth2', 'b', 'success')
    recordActivity('DELETE', '/v1/person/x', 'c', 'success')
    const rows = queryActivityLog()
    expect(rows).toHaveLength(3)
    // Newest is the third recorded.
    expect(rows[0].path).toBe('/v1/person/x')
    expect(rows[2].path).toBe('/v1/group')
  })
})

describe('queryActivityLog filters', () => {
  beforeEach(() => {
    recordActivity('POST', '/v1/person', 'alice', 'success')
    recordActivity('POST', '/v1/group', 'alice', 'error', 'boom')
    recordActivity('DELETE', '/v1/person/x', 'bob', 'success')
  })

  it('filters by actor', () => {
    expect(queryActivityLog({ actor: 'alice' })).toHaveLength(2)
    expect(queryActivityLog({ actor: 'bob' })).toHaveLength(1)
    expect(queryActivityLog({ actor: 'nobody' })).toHaveLength(0)
  })

  it('filters by result', () => {
    expect(queryActivityLog({ result: 'success' })).toHaveLength(2)
    expect(queryActivityLog({ result: 'error' })).toHaveLength(1)
  })

  it('honors limit and offset for paging', () => {
    const page1 = queryActivityLog({ limit: 1, offset: 0 })
    const page2 = queryActivityLog({ limit: 1, offset: 1 })
    expect(page1).toHaveLength(1)
    expect(page2).toHaveLength(1)
    expect(page1[0].id).not.toBe(page2[0].id)
  })

  it('combines actor + result', () => {
    const rows = queryActivityLog({ actor: 'alice', result: 'error' })
    expect(rows).toHaveLength(1)
    expect(rows[0].path).toBe('/v1/group')
  })
})

describe('action derivation from method+path', () => {
  it.each([
    ['POST', '/v1/person', 'Criar person'],
    ['DELETE', '/v1/person/foo', 'Excluir person'],
    ['PUT', '/v1/group/admins/_attr/description', 'Atualizar group'],
    ['POST', '/v1/group/admins/_attr/member', 'Adicionar membro'],
    ['DELETE', '/v1/group/admins/_attr/member', 'Remover membro'],
    ['POST', '/v1/service_account/sa/_api_token', 'Gerar token'],
    ['DELETE', '/v1/service_account/sa/_api_token/abc', 'Revogar token'],
    ['POST', '/v1/recycle_bin/uuid/_revive', 'Restaurar da lixeira'],
  ])('%s %s → %s', (method, path, action) => {
    recordActivity(method, path, 'tester', 'success')
    const [row] = queryActivityLog({ limit: 1 })
    expect(row.action).toBe(action)
  })
})
