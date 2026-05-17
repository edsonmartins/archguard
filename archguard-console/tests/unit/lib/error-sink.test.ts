import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  reportError,
  subscribeErrors,
  _clearSubscribers,
} from '@/lib/utils/error-sink'

beforeEach(() => {
  _clearSubscribers()
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

describe('error-sink', () => {
  it('always logs to console', () => {
    reportError(new Error('boom'), { source: 'test' })
    expect(console.error).toHaveBeenCalledOnce()
  })

  it('forwards to every subscriber', () => {
    const a = vi.fn()
    const b = vi.fn()
    subscribeErrors(a)
    subscribeErrors(b)
    reportError(new Error('x'), { tag: 'unit' })
    expect(a).toHaveBeenCalledOnce()
    expect(b).toHaveBeenCalledOnce()
    const [err, ctx] = a.mock.calls[0]!
    expect(err).toBeInstanceOf(Error)
    expect(ctx).toMatchObject({ tag: 'unit' })
  })

  it('wraps non-Error values into Error', () => {
    const sub = vi.fn()
    subscribeErrors(sub)
    reportError('string error')
    const [err] = sub.mock.calls[0]!
    expect(err).toBeInstanceOf(Error)
    expect((err as Error).message).toBe('string error')
  })

  it('does not let a broken subscriber kill other subscribers', () => {
    const broken = () => {
      throw new Error('subscriber crash')
    }
    const ok = vi.fn()
    subscribeErrors(broken)
    subscribeErrors(ok)
    expect(() => reportError(new Error('x'))).not.toThrow()
    expect(ok).toHaveBeenCalledOnce()
  })

  it('unsubscribes when the returned dispose is called', () => {
    const sub = vi.fn()
    const dispose = subscribeErrors(sub)
    reportError(new Error('1'))
    dispose()
    reportError(new Error('2'))
    expect(sub).toHaveBeenCalledOnce()
  })
})
