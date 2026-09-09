import { beforeEach, expect, it, vi } from 'vitest'
import type { SessionData } from '@/server/auth'
const m = vi.hoisted(() => ({ groups: vi.fn(), logs: vi.fn(), sessions: vi.fn(), current: vi.fn() }))
vi.mock('@/server/idp', () => ({ getUserGroups: m.groups }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({
  handler: (fn: unknown) => fn,
  inputValidator: () => ({ handler: (fn: unknown) => fn }),
}) }))
vi.mock('@/server/activity-log', () => ({ queryActivityLog: m.logs }))
vi.mock('@/server/warpgate-proxy', () => ({ listWarpgateSessions: m.sessions, warpgateConfigured: () => true }))
vi.mock('@/server/session-guard', async (original) => ({
  ...await original<typeof import('@/server/session-guard')>(), requireSession: m.current,
}))
import { assertPrincipalTenantAccess } from '@/server/session-guard'
import { getActivityLogFn } from '@/server/activity-log-fn'
import { listUnifiedAuditFn } from '@/server/unified-audit-fn'
const tenantAdmin = {
  groups: ['tenant_a', 'archguard_tenant_admins'], permissions: ['persons:update', 'audit:read'],
} as SessionData
beforeEach(() => {
  vi.resetAllMocks()
  m.current.mockReturnValue(tenantAdmin)
  m.logs.mockReturnValue([])
  m.sessions.mockResolvedValue([])
})
it.each([
  ['shared identity', ['tenant_a', 'tenant_b']],
  ['unassigned identity', []],
  ['platform administrator', ['tenant_a', 'archgate/archguard_super_admins']],
  ['foreign tenant', ['tenant_b']],
  ['unavailable identity', null],
])('denies %s before granting authority', async (_, groups) => {
  m.groups.mockResolvedValue(groups)
  await expect(assertPrincipalTenantAccess('target', tenantAdmin)).rejects.toThrow('Forbidden')
})
it('allows a non-privileged identity wholly within the administered tenant', async () => {
  m.groups.mockResolvedValue(['archgate/tenant_a', 'archguard_users'])
  await expect(assertPrincipalTenantAccess('target', tenantAdmin)).resolves.toBeUndefined()
})
it('preserves explicit platform authority', async () => {
  await expect(assertPrincipalTenantAccess('target', { ...tenantAdmin, permissions: ['system:admin'] }))
    .resolves.toBeUndefined()
})
it.each(['audit:read', 'persons:read', 'gateways:read'] as const)('denies global audit to %s before reading data', async (permission) => {
  m.current.mockReturnValue({ ...tenantAdmin, permissions: [permission] })
  const timeline = listUnifiedAuditFn as unknown as (ctx: { data: { limit: number; source: string } }) => Promise<unknown>
  const activity = getActivityLogFn as unknown as () => Promise<unknown>
  await expect(timeline({ data: { limit: 100, source: 'all' } })).rejects.toThrow('Forbidden')
  await expect(activity()).rejects.toThrow('Forbidden')
  expect(m.logs).not.toHaveBeenCalled()
  expect(m.sessions).not.toHaveBeenCalled()
})
it('allows platform administrator to read global audit', async () => {
  m.current.mockReturnValue({ ...tenantAdmin, permissions: ['system:admin'] })
  const timeline = listUnifiedAuditFn as unknown as (ctx: { data: { limit: number; source: string } }) => Promise<unknown>
  await expect(timeline({ data: { limit: 100, source: 'all' } })).resolves.toMatchObject({ events: [] })
  expect(m.logs).toHaveBeenCalled()
  expect(m.sessions).toHaveBeenCalled()
})
