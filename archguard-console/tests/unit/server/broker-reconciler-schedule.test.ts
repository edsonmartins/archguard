import { afterEach, expect, it, vi } from 'vitest'

afterEach(() => {
  vi.unstubAllEnvs()
  vi.useRealTimers()
  vi.resetModules()
})

it('does not schedule lease reconciliation unless explicitly enabled', async () => {
  vi.useFakeTimers()
  vi.stubEnv('BROKER_LEASE_RECONCILER_ENABLED', '')
  const spy = vi.spyOn(globalThis, 'setInterval')
  await import('@/server/broker-session')
  expect(spy).not.toHaveBeenCalled()
  spy.mockRestore()
})

it('schedules a bounded reconciliation loop when enabled', async () => {
  vi.useFakeTimers()
  vi.stubEnv('BROKER_LEASE_RECONCILER_ENABLED', '1')
  vi.stubEnv('BROKER_LEASE_RECONCILER_INTERVAL_MS', '1000')
  const spy = vi.spyOn(globalThis, 'setInterval')
  await import('@/server/broker-session')
  expect(spy).toHaveBeenCalledWith(expect.any(Function), 5000)
  spy.mockRestore()
})
