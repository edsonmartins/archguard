import { describe, it, expect, beforeEach, vi } from 'vitest'

const ipMock = vi.fn(() => '10.0.0.1')

vi.mock('@tanstack/react-start/server', () => ({
  getRequestIP: ipMock,
}))

vi.mock('@/server/logger', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}))

let enforceRateLimit: typeof import('@/server/rate-limit').enforceRateLimit

beforeEach(async () => {
  vi.resetModules()
  ipMock.mockReturnValue('10.0.0.1')
  ;({ enforceRateLimit } = await import('@/server/rate-limit'))
})

describe('rate-limit sliding window', () => {
  it('allows up to limit calls within the window', () => {
    for (let i = 0; i < 5; i++) {
      expect(() => enforceRateLimit('test', 5, 1000)).not.toThrow()
    }
  })

  it('rejects the call that crosses the limit', () => {
    for (let i = 0; i < 5; i++) enforceRateLimit('test', 5, 1000)
    expect(() => enforceRateLimit('test', 5, 1000)).toThrow(/Too many requests/)
  })

  it('lets calls through again once the window slides past', async () => {
    vi.useFakeTimers()
    try {
      const t0 = Date.now()
      vi.setSystemTime(t0)
      for (let i = 0; i < 3; i++) enforceRateLimit('test', 3, 1000)
      expect(() => enforceRateLimit('test', 3, 1000)).toThrow()

      vi.setSystemTime(t0 + 1500)
      expect(() => enforceRateLimit('test', 3, 1000)).not.toThrow()
    } finally {
      vi.useRealTimers()
    }
  })

  it('isolates buckets per scope', () => {
    for (let i = 0; i < 5; i++) enforceRateLimit('login', 5, 1000)
    expect(() => enforceRateLimit('login', 5, 1000)).toThrow()
    // proxy bucket is still empty
    expect(() => enforceRateLimit('proxy', 5, 1000)).not.toThrow()
  })

  it('isolates buckets per IP', () => {
    ipMock.mockReturnValue('10.0.0.1')
    for (let i = 0; i < 3; i++) enforceRateLimit('test', 3, 1000)
    expect(() => enforceRateLimit('test', 3, 1000)).toThrow()

    ipMock.mockReturnValue('10.0.0.2')
    expect(() => enforceRateLimit('test', 3, 1000)).not.toThrow()
  })

  it('falls back to unknown bucket when IP cannot be resolved', () => {
    ipMock.mockImplementation(() => {
      throw new Error('no request context')
    })
    expect(() => enforceRateLimit('test', 2, 1000)).not.toThrow()
    expect(() => enforceRateLimit('test', 2, 1000)).not.toThrow()
    expect(() => enforceRateLimit('test', 2, 1000)).toThrow()
  })
})
