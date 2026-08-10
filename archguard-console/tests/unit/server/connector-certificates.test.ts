import { afterEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests, getDb } from '@/server/db'
import {
  markConnectorCertificatesRevoked,
  registerConnectorCertificate,
} from '@/server/connector-enrollment'

describe('connector certificate lifecycle', () => {
  let dir: string | undefined

  afterEach(() => {
    _resetDbForTests()
    if (dir) rmSync(dir, { recursive: true, force: true })
    dir = undefined
  })

  it('registers a rotated certificate and revokes only active serials', () => {
    dir = mkdtempSync(join(tmpdir(), 'archgate-certs-'))
    process.env.ARCHGUARD_DB_PATH = join(dir, 'console.sqlite')
    registerConnectorCertificate({ serial_number: 'serial-old', connector_id: 'connector-lab', site_slug: 'site-lab' })
    registerConnectorCertificate({ serial_number: 'serial-new', connector_id: 'connector-lab', site_slug: 'site-lab' })
    const revoked = markConnectorCertificatesRevoked('connector-lab')
    expect(revoked).toBe(2)
    const rows = getDb().prepare('SELECT serial_number, status, revoked_at FROM connector_certificates ORDER BY serial_number').all() as Array<{ serial_number: string; status: string; revoked_at: string | null }>
    expect(rows).toHaveLength(2)
    expect(rows.every((row) => row.status === 'revoked' && row.revoked_at)).toBe(true)
    expect(markConnectorCertificatesRevoked('connector-lab')).toBe(0)
  })
})
