import { afterEach, expect, it, vi } from 'vitest'

afterEach(() => {
  vi.unstubAllEnvs()
  vi.useRealTimers()
  vi.resetModules()
})

it('does not schedule a forwarder without an explicit destination', async () => {
  vi.useFakeTimers()
  vi.stubEnv('AUDIT_OUTBOX_FORWARDER_URL', '')
  const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
  await import('@/server/audit-forwarder')
  expect(setIntervalSpy).not.toHaveBeenCalled()
  setIntervalSpy.mockRestore()
})

it('schedules a bounded periodic retry when configured', async () => {
  vi.useFakeTimers()
  vi.stubEnv('AUDIT_OUTBOX_FORWARDER_URL', 'https://audit.test/events')
  vi.stubEnv('AUDIT_OUTBOX_FORWARDER_INTERVAL_MS', '1000')
  const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
  const module = await import('@/server/audit-forwarder')
  expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 5000)
  module.startAuditForwarder()
  expect(setIntervalSpy).toHaveBeenCalledTimes(1)
  setIntervalSpy.mockRestore()
})
