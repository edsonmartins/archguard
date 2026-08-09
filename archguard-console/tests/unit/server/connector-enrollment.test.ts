import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '@/server/db'
import { consumeConnectorEnrollment, issueConnectorEnrollment } from '@/server/connector-enrollment'

describe('connector enrollment', () => {
  let dir: string | undefined

  afterEach(() => {
    _resetDbForTests()
    if (dir) rmSync(dir, { recursive: true, force: true })
    dir = undefined
  })

  it('issues a token that can be consumed exactly once', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-enrollment-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    const issued = issueConnectorEnrollment({
      site_slug: 'rio_quality_lab',
      connector_id: 'connector-lab',
      created_by: 'operator-1',
      ttl_seconds: 60,
    })

    expect(issued.token).not.toContain(issued.enrollment.id)
    expect(consumeConnectorEnrollment(issued.token)).toMatchObject({
      id: issued.enrollment.id,
      site_slug: 'rio_quality_lab',
      connector_id: 'connector-lab',
    })
    expect(consumeConnectorEnrollment(issued.token)).toBeNull()
  })

  it('fails closed for an unknown token', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-enrollment-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    issueConnectorEnrollment({
      site_slug: 'rio_quality_lab',
      connector_id: 'connector-lab',
      created_by: 'operator-1',
    })
    expect(consumeConnectorEnrollment('invalid-token-that-is-never-issued')).toBeNull()
  })
})
