import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { SessionData } from '@/server/auth'

const m = vi.hoisted(() => ({ current: vi.fn(), fetch: vi.fn(), activity: vi.fn() }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({
  inputValidator: () => ({ handler: (fn: unknown) => fn }),
}) }))
vi.mock('@/server/session-guard', async (original) => ({
  ...await original<typeof import('@/server/session-guard')>(), requireSession: m.current,
}))
vi.mock('@/server/activity-log', () => ({ recordActivity: m.activity, getActor: () => 'operator' }))
vi.mock('@/server/rate-limit', () => ({ enforceRateLimit: vi.fn() }))
import { archguardApiFn } from '@/server/archguard-proxy'

const invoke = archguardApiFn as unknown as (ctx: { data: { method: string; path: string; body?: unknown } }) => Promise<unknown>
beforeEach(() => {
  vi.resetAllMocks()
  vi.stubGlobal('fetch', m.fetch)
  m.current.mockReturnValue({ groups: ['tenant_a'], permissions: [
    'persons:read', 'persons:create', 'persons:update', 'persons:delete', 'persons:credentials',
    'groups:read', 'groups:members', 'oauth2:read', 'service_accounts:tokens', 'settings:update',
  ] } as SessionData)
  m.fetch.mockResolvedValue(new Response('{}'))
})
afterEach(() => vi.unstubAllGlobals())

it.each([
  ['GET', '/v1/person'],
  ['GET', '/v1/person/foreign'],
  ['POST', '/v1/person'],
  ['PATCH', '/v1/person/foreign'],
  ['DELETE', '/v1/person/foreign'],
  ['POST', '/v1/person/foreign/_credential/_update_intent/1h'],
  ['GET', '/v1/group'],
  ['POST', '/v1/group/admins/_attr/member'],
  ['GET', '/v1/oauth2/app/_basic_secret'],
  ['POST', '/v1/service_account/sa/_api_token'],
  ['PUT', '/v1/system'],
  ['POST', '/v1/recycle_bin/id/_revive'],
  ['POST', '/status'],
  ['GET', '/status/private'],
])('denies delegated %s %s before the global upstream call', async (method, path) => {
  await expect(invoke({ data: { method, path } })).rejects.toThrow('proxy global exige administrador')
  expect(m.fetch).not.toHaveBeenCalled()
  expect(m.activity).not.toHaveBeenCalled()
})

it('preserves the exact read-only health endpoint', async () => {
  await expect(invoke({ data: { method: 'GET', path: '/status' } })).resolves.toEqual({})
  expect(m.fetch).toHaveBeenCalledOnce()
})

it('allows explicit platform administration', async () => {
  m.current.mockReturnValue({ groups: [], permissions: ['system:admin'] })
  await expect(invoke({ data: { method: 'POST', path: '/v1/group/admins/_attr/member', body: ['alice'] } })).resolves.toEqual({})
  expect(m.fetch).toHaveBeenCalledOnce()
  expect(m.activity).toHaveBeenCalledOnce()
})

it('keeps the path allowlist for platform administrators', async () => {
  m.current.mockReturnValue({ groups: [], permissions: ['system:admin'] })
  await expect(invoke({ data: { method: 'GET', path: '/internal' } })).rejects.toThrow('Forbidden path')
  expect(m.fetch).not.toHaveBeenCalled()
})
