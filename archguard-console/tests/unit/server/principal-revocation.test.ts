import { afterEach, expect, it, vi } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '@/server/db'
import { revokePrincipal, isPrincipalRevoked, reactivatePrincipal, isPrincipalSessionRevoked } from '@/server/principal-revocation'

let dir: string | undefined
afterEach(() => {
  vi.useRealTimers()
  _resetDbForTests()
  if (dir) rmSync(dir, { recursive: true, force: true })
})

it('reactivation keeps old and missing authentication times denied across restart', () => {
  dir = mkdtempSync(join(tmpdir(), 'archgate-reactivation-'))
  const path = join(dir, 'console.sqlite')
  _resetDbForTests(path)
  vi.useFakeTimers()
  vi.setSystemTime(2000000)
  revokePrincipal('alice')
  reactivatePrincipal('alice')
  _resetDbForTests(path)
  expect(isPrincipalRevoked('alice')).toBe(false)
  expect(isPrincipalSessionRevoked('alice', 1999)).toBe(true)
  expect(isPrincipalSessionRevoked('alice', 2000)).toBe(true)
  expect(isPrincipalSessionRevoked('alice')).toBe(true)
  expect(isPrincipalSessionRevoked('alice', 2001)).toBe(false)
  expect(isPrincipalSessionRevoked('bob')).toBe(false)
  revokePrincipal('alice')
  expect(isPrincipalSessionRevoked('alice', 2001)).toBe(true)
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
