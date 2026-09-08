import { beforeEach, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ close: vi.fn(), revoke: vi.fn(), get: vi.fn(), mark: vi.fn() }))
vi.mock('@/server/rustguac-proxy', () => ({ closeRustGuacSession: mocks.close }))
vi.mock('@/server/openbao-proxy', () => ({ revokeLease: mocks.revoke }))
vi.mock('@/server/db', () => ({ getBrokerSession: mocks.get, closeBrokerSession: mocks.mark }))
import { closeBrokerSessionAndLease } from '@/server/broker-session'
import { offboardingResult } from '@/server/offboarding-result'
beforeEach(() => {
  vi.resetAllMocks()
  mocks.get.mockReturnValue({ lease_id: 'database/creds/a/one', closed_at: null })
})
it('revokes the exact lease even if gateway cleanup fails and permits retry', async () => {
  mocks.close.mockRejectedValueOnce(new Error('offline'))
  await expect(closeBrokerSessionAndLease('session-a')).rejects.toThrow('incomplete')
  expect(mocks.revoke).toHaveBeenCalledWith('database/creds/a/one')
  expect(mocks.mark).not.toHaveBeenCalled()
  await closeBrokerSessionAndLease('session-a')
  expect(mocks.mark).toHaveBeenCalledWith('session-a')
})
it('does not mark cleanup complete when lease revocation fails', async () => {
  mocks.revoke.mockRejectedValueOnce(new Error('offline'))
  await expect(closeBrokerSessionAndLease('session-a')).rejects.toThrow('incomplete')
  expect(mocks.close).toHaveBeenCalledWith('session-a')
  expect(mocks.mark).not.toHaveBeenCalled()
})
it('rejects unknown sessions before external calls', async () => {
  mocks.get.mockReturnValue(undefined)
  await expect(closeBrokerSessionAndLease('unknown')).rejects.toThrow('Unknown')
  expect(mocks.close).not.toHaveBeenCalled()
  expect(mocks.revoke).not.toHaveBeenCalled()
})
it('reports blocked login separately from incomplete revocation', () => {
  expect(offboardingResult([{ component: 'idp', ok: true }, { component: 'openbao', ok: false }]))
    .toEqual({ ok: false, all_ok: false, login_blocked: true })
})
