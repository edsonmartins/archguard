import { afterEach, beforeEach, expect, it, vi } from 'vitest'
const m = vi.hoisted(() => ({ current: vi.fn(), groups: vi.fn(), fetch: vi.fn(), audit: vi.fn() }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({
  inputValidator: (parse: (input: unknown) => unknown) => ({
    handler: (fn: (ctx: { data: unknown }) => unknown) => (ctx: { data: unknown }) => fn({ data: parse(ctx.data) }),
  }),
}) }))
vi.mock('@/server/session-guard', async (original) => ({
  ...await original<typeof import('@/server/session-guard')>(), requireSession: m.current,
}))
vi.mock('@/server/idp', () => ({ getUserGroups: m.groups }))
vi.mock('@/server/activity-log', () => ({ recordActivity: m.audit }))
vi.mock('@/server/http-integration-client', () => ({ integrationFetch: m.fetch }))
vi.mock('@/server/rate-limit', () => ({ enforceRateLimit: vi.fn() }))
import { resetPersonCredentialFn } from '@/server/person-credential-fn'
const invoke = resetPersonCredentialFn as unknown as (ctx: { data: { id: string; ttl?: number } }) => Promise<unknown>
beforeEach(() => {
  vi.resetAllMocks()
  vi.stubEnv('ARCHGUARD_ID_URL', 'https://identity.test')
  vi.stubEnv('ARCHGUARD_SA_TOKEN', 'service-test')
  m.current.mockReturnValue({ groups: ['tenant_a'], permissions: ['persons:credentials'], user: { name: 'admin-a' } })
  m.groups.mockResolvedValue(['tenant_a'])
  m.fetch.mockResolvedValueOnce(new Response(JSON.stringify({ attrs: { uuid: ['id-1'], name: ['alice'] } })))
  m.fetch.mockResolvedValue(new Response(JSON.stringify({ token: 'synthetic-reset-token', privateField: 'not-returned' })))
})
afterEach(() => vi.unstubAllEnvs())

it('resolves the UUID and validates tenant authority before issuing a reset', async () => {
  await expect(invoke({ data: { id: 'id-1' } })).resolves.toEqual({ token: 'synthetic-reset-token' })
  expect(m.groups).toHaveBeenCalledWith('alice')
  expect(m.fetch.mock.calls[1][0]).toBe('https://identity.test/v1/person/id-1/_credential/_update_intent/3600')
  expect(m.groups.mock.invocationCallOrder[0]).toBeLessThan(m.fetch.mock.invocationCallOrder[1])
  expect(JSON.stringify(m.audit.mock.calls)).not.toContain('synthetic-reset-token')
})
it.each([
  { groups: ['tenant_b'] }, { groups: ['tenant_a', 'tenant_b'] },
  { groups: [] }, { groups: ['tenant_a', 'archguard_super_admins'] }, { groups: null },
])(
  'denies foreign, shared, unassigned, privileged or unknown ownership: %j', async ({ groups }) => {
    m.groups.mockResolvedValue(groups)
    await expect(invoke({ data: { id: 'id-1' } })).rejects.toThrow('Forbidden')
    expect(m.fetch).toHaveBeenCalledTimes(1)
  },
)
it('denies update-only authority before reading the identity', async () => {
  m.current.mockReturnValue({ groups: ['tenant_a'], permissions: ['persons:update'] })
  await expect(invoke({ data: { id: 'id-1' } })).rejects.toThrow('Forbidden')
  expect(m.fetch).not.toHaveBeenCalled()
})
it('denies an unauthenticated caller before reading the identity', async () => {
  m.current.mockImplementation(() => { throw new Error('Unauthorized') })
  await expect(invoke({ data: { id: 'id-1' } })).rejects.toThrow('Unauthorized')
  expect(m.fetch).not.toHaveBeenCalled()
})
it('rejects an identity response unrelated to the route identifier', async () => {
  await expect(invoke({ data: { id: 'other-id' } })).rejects.toThrow('Identity mismatch')
  expect(m.groups).not.toHaveBeenCalled()
  expect(m.fetch).toHaveBeenCalledTimes(1)
})
it.each([{ id: '..' }, { id: 'a/b' }, { id: 'id-1', ttl: 0 }, { id: 'id-1', ttl: 604801 }])(
  'rejects malformed input: %j', async (data) => {
    await expect(async () => invoke({ data })).rejects.toThrow()
    expect(m.fetch).not.toHaveBeenCalled()
  },
)
it('allows a platform administrator with an authoritative identity response', async () => {
  m.current.mockReturnValue({ groups: [], permissions: ['system:admin'] })
  await expect(invoke({ data: { id: 'id-1' } })).resolves.toEqual({ token: 'synthetic-reset-token' })
  expect(m.groups).not.toHaveBeenCalled()
})
it('does not expose an upstream error body in errors or audit', async () => {
  m.fetch.mockReset().mockResolvedValue(new Response('synthetic-private-error', { status: 500 }))
  await expect(invoke({ data: { id: 'id-1' } })).rejects.toThrow('Identity credential operation failed')
  expect(m.fetch).toHaveBeenCalledTimes(1)
  expect(JSON.stringify(m.audit.mock.calls)).not.toContain('synthetic-private-error')
})
