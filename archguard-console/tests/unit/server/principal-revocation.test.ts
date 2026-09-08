import { afterEach, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '@/server/db'
import { revokePrincipal, isPrincipalRevoked } from '@/server/principal-revocation'

let dir: string | undefined
afterEach(() => {
  _resetDbForTests()
  if (dir) rmSync(dir, { recursive: true, force: true })
})

it('keeps the selected principal blocked across restart without blocking another user', () => {
  dir = mkdtempSync(join(tmpdir(), 'archgate-revocation-'))
  const path = join(dir, 'console.sqlite')
  _resetDbForTests(path)
  revokePrincipal('alice')
  _resetDbForTests(path)
  expect(isPrincipalRevoked('alice')).toBe(true)
  expect(isPrincipalRevoked('bob')).toBe(false)
})
