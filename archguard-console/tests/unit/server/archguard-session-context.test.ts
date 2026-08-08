import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const fetchMock = vi.fn()

describe('ArchGuard session-context', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubGlobal('fetch', fetchMock)
    process.env.ORCHESTRATION_URL = 'http://orch:8090'
    process.env.ORCH_API_TOKEN = 'orch-secret'
    fetchMock.mockReset()
  })

  afterEach(() => vi.unstubAllGlobals())

  it('resolves active memberships with service authentication', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          subject: 'sub-1',
          identity_id: 'identity-1',
          identity_status: 'active',
          memberships: [
            { membership_id: 'membership-1', organization_id: 'org-1', status: 'active' },
          ],
        }),
        { status: 200 },
      ),
    )
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    const result = await resolveArchGuardSessionContext('sub-1')
    expect(result.memberships).toHaveLength(1)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://orch:8090/orchestration/v1/identities/session-context')
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer orch-secret')
    expect(init.body).toBe(JSON.stringify({ subject: 'sub-1' }))
  })

  it('rejects inactive identities and subject mismatches', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ subject: 'other', identity_status: 'active' }), { status: 200 }),
    )
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('subject mismatch')

    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ subject: 'sub-1', identity_status: 'disabled' }), { status: 200 }),
    )
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('not active')
  })
})
