import { afterEach, beforeEach, expect, it, vi } from 'vitest'
const m = vi.hoisted(() => ({ current: vi.fn(), groups: vi.fn(), fetch: vi.fn() }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({
  handler: (fn: unknown) => fn,
  inputValidator: (parse: (input: unknown) => unknown) => ({
    handler: (fn: (ctx: { data: unknown }) => unknown) => (ctx: { data: unknown }) => fn({ data: parse(ctx.data) }),
  }),
}) }))
vi.mock('@/server/session-guard', async (original) => ({
  ...await original<typeof import('@/server/session-guard')>(), requireSession: m.current,
}))
vi.mock('@/server/idp', async () => ({
  ...await import('@/server/idp/groups'), getUserGroups: m.groups,
}))
vi.mock('@/server/http-integration-client', () => ({ integrationFetch: m.fetch }))
vi.mock('@/server/rate-limit', () => ({ enforceRateLimit: vi.fn() }))
import { getPersonFn, listPersonsFn } from '@/server/person-read-fn'
const list = listPersonsFn as unknown as () => Promise<Array<{ attrs: Record<string, string[]> }>>
const get = getPersonFn as unknown as (ctx: { data: { id: string } }) => Promise<unknown>
const person = (name: string) => ({ attrs: { uuid: [`id-${name}`], name: [name], memberof: ['tenant_a'], password: ['private'] } })
beforeEach(() => {
  vi.resetAllMocks()
  vi.stubEnv('ARCHGUARD_ID_URL', 'https://identity.test')
  vi.stubEnv('ARCHGUARD_SA_TOKEN', 'service-test')
  m.current.mockReturnValue({ groups: ['tenant_a'], permissions: ['persons:read'] })
  m.groups.mockResolvedValue(['tenant_a'])
})
afterEach(() => vi.unstubAllEnvs())

it('filters before returning data and uses authoritative memberships, not list claims', async () => {
  const memberships: Record<string, string[]> = {
    alice: ['archgate/tenant_a'], bob: ['tenant_b'], shared: ['tenant_a', 'tenant_b'],
    privileged: ['tenant_a', 'archguard_super_admins'], unassigned: [],
  }
  m.fetch.mockResolvedValue(new Response(JSON.stringify(Object.keys(memberships).map(person))))
  m.groups.mockImplementation(async (name: string) => memberships[name])
  const result = await list()
  expect(result.map((entry) => entry.attrs.name[0])).toEqual(['alice'])
  expect(result[0].attrs.memberof).toEqual(['tenant_a'])
  expect(JSON.stringify(result)).not.toContain('private')
})
it('allows the exact authorized detail ID', async () => {
  m.fetch.mockResolvedValue(new Response(JSON.stringify(person('alice'))))
  const result = await get({ data: { id: 'id-alice' } })
  expect(result).toMatchObject({ attrs: { name: ['alice'] } })
  expect(m.groups).toHaveBeenCalledWith('alice')
})
it('denies direct lookup of a known foreign ID', async () => {
  m.fetch.mockResolvedValue(new Response(JSON.stringify(person('bob'))))
  m.groups.mockResolvedValue(['tenant_b'])
  await expect(get({ data: { id: 'id-bob' } })).rejects.toThrow('Identity unavailable')
})
it('rejects a detail response for another identity', async () => {
  m.fetch.mockResolvedValue(new Response(JSON.stringify(person('bob'))))
  await expect(get({ data: { id: 'id-alice' } })).rejects.toThrow('Identity unavailable')
  expect(m.groups).not.toHaveBeenCalled()
})
it('fails the whole list when membership evidence is unavailable', async () => {
  m.fetch.mockResolvedValue(new Response(JSON.stringify([person('alice'), person('bob')])) )
  m.groups.mockImplementation(async (name: string) => name === 'alice' ? ['tenant_a'] : null)
  await expect(list()).rejects.toThrow('verification unavailable')
})
it.each([
  { groups: [], permissions: ['persons:read'] },
  { groups: ['tenant_a'], permissions: ['persons:update'] },
])('denies missing tenant or permission before reading inventory: %j', async (session) => {
  m.current.mockReturnValue(session)
  await expect(list()).rejects.toThrow('Forbidden')
  expect(m.fetch).not.toHaveBeenCalled()
})
it('preserves platform inventory access but strips unknown attributes', async () => {
  m.current.mockReturnValue({ groups: [], permissions: ['system:admin'] })
  m.fetch.mockResolvedValue(new Response(JSON.stringify([person('alice'), person('bob')])) )
  const result = await list()
  expect(result).toHaveLength(2)
  expect(JSON.stringify(result)).not.toContain('private')
  expect(m.groups).not.toHaveBeenCalled()
})
it.each([
  { label: 'malformed', payload: [{}] },
  { label: 'duplicate', payload: [person('alice'), person('alice')] },
  { label: 'oversized', payload: Array.from({ length: 1001 }, (_, i) => person(String(i))) },
])('rejects $label inventories without checking memberships', async ({ payload }) => {
  m.fetch.mockResolvedValue(new Response(JSON.stringify(payload)))
  await expect(list()).rejects.toThrow()
  expect(m.groups).not.toHaveBeenCalled()
})
it('does not expose an upstream HTTP error body', async () => {
  m.fetch.mockResolvedValue(new Response('private-upstream', { status: 500 }))
  await expect(list()).rejects.toThrow('Identity query unavailable')
})
