import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '@/server/db'
import {
  claimIdempotency,
  completeIdempotency,
  hashBody,
  IdempotencyConflict,
} from '@/server/bff-idempotency'

describe('BFF idempotency repository', () => {
  let dir: string | undefined

  afterEach(() => {
    _resetDbForTests()
    if (dir) rmSync(dir, { recursive: true, force: true })
    dir = undefined
  })

  it('claims once and replays the completed response', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-idempotency-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    const bodyHash = hashBody({ name: 'site-a' })
    expect(claimIdempotency('org-a:actor-a:POST:/sites:key-1', bodyHash)).toBeNull()
    completeIdempotency('org-a:actor-a:POST:/sites:key-1', 201, { id: 'site-a' })
    expect(claimIdempotency('org-a:actor-a:POST:/sites:key-1', bodyHash)).toMatchObject({
      statusCode: 201,
      response: { id: 'site-a' },
      completed: true,
    })
  })

  it('rejects reuse with a different body', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-idempotency-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    claimIdempotency('org-a:actor-a:POST:/sites:key-1', hashBody({ name: 'site-a' }))
    expect(() => claimIdempotency('org-a:actor-a:POST:/sites:key-1', hashBody({ name: 'site-b' })))
      .toThrow(IdempotencyConflict)
  })

  it('never persists a one-time secret in either response column', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-idempotency-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    const hash = hashBody({ reason: 'test' })
    claimIdempotency('checkout:test', hash)
    completeIdempotency('checkout:test', 200, { secret: 'synthetic-private-value' }, {
      statusCode: 409, response: { error: 'already revealed' },
    })
    expect(JSON.stringify(getDb().prepare('SELECT * FROM bff_idempotency').all()))
      .not.toContain('synthetic-private-value')
    expect(claimIdempotency('checkout:test', hash)).toMatchObject({
      statusCode: 409, response: { error: 'already revealed' },
    })
  })
})
