import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const fetchMock = vi.fn()

describe('ArchGuard session-context', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubEnv('ORCHESTRATION_URL', 'http://orch:8090')
    vi.stubEnv('ORCH_API_TOKEN', 'orch-secret')
    fetchMock.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
  })

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
      new Response(JSON.stringify({ subject: 'other', identity_id: 'identity-1', identity_status: 'active', memberships: [] }), { status: 200 }),
    )
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('subject mismatch')

    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ subject: 'sub-1', identity_id: 'identity-1', identity_status: 'disabled', memberships: [] }), { status: 200 }),
    )
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('not active')
  })

  it.each([
    null,
    {},
    { identity_id: '' },
    { identity_id: ' identity-1' },
    { memberships: null },
    { memberships: {} },
    { memberships: [{ membership_id: 'm-1', status: 'active' }] },
    { memberships: [{ membership_id: 'm-1', organization_id: 'org-1', status: true }] },
  ])('rejects malformed context: %j', async (override) => {
    const valid = { subject: 'sub-1', identity_id: 'identity-1', identity_status: 'active', memberships: [] }
    const payload = override === null || Object.keys(override).length === 0 ? override : { ...valid, ...override }
    fetchMock.mockResolvedValue(new Response(JSON.stringify(payload)))
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('Invalid ArchGuard session context')
  })

  it('excludes inactive and unknown membership statuses', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({
      subject: 'sub-1', identity_id: 'identity-1', identity_status: 'active',
      memberships: ['active', 'disabled', 'pending', 'unknown'].map((status) => ({
        membership_id: status, organization_id: `org-${status}`, status,
      })),
    })))
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    const result = await resolveArchGuardSessionContext('sub-1')
    expect(result.memberships.map((membership) => membership.membership_id)).toEqual(['active'])
  })

  it('rejects invalid JSON without echoing upstream content', async () => {
    fetchMock.mockResolvedValue(new Response('private upstream error'))
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    await expect(resolveArchGuardSessionContext('sub-1')).rejects.toThrow('Invalid ArchGuard session context')
  })

  it('rejects an empty subject before contacting the control plane', async () => {
    const { resolveArchGuardSessionContext } = await import('@/server/archguard-session-context')
    await expect(resolveArchGuardSessionContext(' ')).rejects.toThrow('Invalid authenticated subject')
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
