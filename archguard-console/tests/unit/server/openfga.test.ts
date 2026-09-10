import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const fetchMock = vi.fn()

describe('OpenFGA authorization check', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubGlobal('fetch', fetchMock)
    process.env.OPENFGA_ENABLED = '1'
    process.env.OPENFGA_URL = 'http://fga:8080'
    process.env.OPENFGA_STORE_ID = 'store-1'
    process.env.OPENFGA_MODEL_ID = 'model-1'
    process.env.OPENFGA_API_TOKEN = 'fga-secret'
    fetchMock.mockReset()
  })

  afterEach(() => vi.unstubAllGlobals())

  it('sends the tuple and accepts only allowed=true', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ allowed: true }), { status: 200 }))
    const { checkOpenFga } = await import('@/server/openfga')
    await expect(checkOpenFga({ user: 'user:sub-1', relation: 'connect', object: 'connection:site:ssh' })).resolves.toBe(true)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://fga:8080/stores/store-1/check')
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer fga-secret')
    expect(String(init.body)).toContain('model-1')
  })

  it('fails closed when the service is unavailable or denies', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ allowed: false }), { status: 200 }))
    const { checkOpenFga } = await import('@/server/openfga')
    await expect(checkOpenFga({ user: 'user:sub-1', relation: 'connect', object: 'connection:x' })).resolves.toBe(false)
    fetchMock.mockResolvedValue(new Response('{}', { status: 503 }))
    await expect(checkOpenFga({ user: 'user:sub-1', relation: 'connect', object: 'connection:x' })).rejects.toThrow('failed')
  })

  it('rejects an enabled but incomplete configuration', async () => {
    delete process.env.OPENFGA_API_TOKEN
    const { checkOpenFga, openFgaConfigured } = await import('@/server/openfga')
    expect(openFgaConfigured()).toBe(false)
    await expect(checkOpenFga({ user: 'user:sub-1', relation: 'connect', object: 'connection:x' })).rejects.toThrow('not configured')
  })

  it('materializes and removes direct grants', async () => {
    fetchMock
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ tuples: [{ key: { user: 'user:sub-1', relation: 'connect', object: 'connection:x' } }] }), { status: 200 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    const { writeOpenFgaGrant, deleteOpenFgaGrantsForUser } = await import('@/server/openfga')
    await expect(writeOpenFgaGrant({ user: 'user:sub-1', relation: 'connect', object: 'connection:x' })).resolves.toBeUndefined()
    await expect(deleteOpenFgaGrantsForUser('user:sub-1')).resolves.toBe(1)
    expect(fetchMock.mock.calls[0][0]).toBe('http://fga:8080/stores/store-1/write')
    expect(String((fetchMock.mock.calls[0][1] as RequestInit).body)).toContain('connection:x')
    expect(fetchMock.mock.calls[2][0]).toBe('http://fga:8080/stores/store-1/write')
    expect(String((fetchMock.mock.calls[2][1] as RequestInit).body)).toContain('deletes')
  })

  it('reads every page and deletes only valid connection tuples in batches', async () => {
    const tuples = Array.from({ length: 101 }, (_, index) => ({
      key: { user: 'user:sub-1', relation: 'connect', object: `connection:site/target-${index}` },
    }))
    fetchMock
      .mockResolvedValueOnce(new Response(JSON.stringify({ tuples: tuples.slice(0, 100), continuation_token: 'next-page' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ tuples: tuples.slice(100) }), { status: 200 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    const { deleteOpenFgaGrantsForUser } = await import('@/server/openfga')
    await expect(deleteOpenFgaGrantsForUser('user:sub-1')).resolves.toBe(101)
    expect(fetchMock).toHaveBeenCalledTimes(4)
    expect(String((fetchMock.mock.calls[1][1] as RequestInit).body)).toContain('next-page')
    expect(String((fetchMock.mock.calls[2][1] as RequestInit).body)).toContain('target-0')
    expect(String((fetchMock.mock.calls[3][1] as RequestInit).body)).toContain('target-100')
  })

  it('fails closed on malformed tuples or repeated pagination', async () => {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ tuples: [{ key: { user: 'user:other', relation: 'connect', object: 'connection:x' } }] }), { status: 200 }))
    const { deleteOpenFgaGrantsForUser } = await import('@/server/openfga')
    await expect(deleteOpenFgaGrantsForUser('user:sub-1')).rejects.toThrow('invalid grant tuple')
    fetchMock.mockReset().mockImplementation(async () => new Response(JSON.stringify({ tuples: [], continuation_token: 'same' }), { status: 200 }))
    await expect(deleteOpenFgaGrantsForUser('user:sub-1')).rejects.toThrow('pagination token repeated')
  })
})
