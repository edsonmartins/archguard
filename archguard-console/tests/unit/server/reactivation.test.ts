import { beforeEach, expect, it, vi } from 'vitest'
const m = vi.hoisted(() => ({ session: vi.fn(), permission: vi.fn(), blocked: vi.fn(), pending: vi.fn(), enable: vi.fn(), activate: vi.fn(), audit: vi.fn() }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({ inputValidator: () => ({ handler: (handler: unknown) => handler }) }) }))
vi.mock('@/server/session-guard', () => ({ requireSession: m.session, requireAnyPerm: m.permission, sessionActor: () => 'admin' }))
vi.mock('@/server/principal-revocation', () => ({ isPrincipalRevoked: m.blocked, reactivatePrincipal: m.activate }))
vi.mock('@/server/db', () => ({ listBrokerSessionsForPrincipal: m.pending }))
vi.mock('@/server/idp/archguard', () => ({ enableArchGuardUser: m.enable }))
vi.mock('@/server/activity-log', () => ({ recordActivity: m.audit }))
import { reactivatePersonFn } from '@/server/reactivation-fn'
const run = reactivatePersonFn as unknown as (input: { data: { username: string } }) => Promise<unknown>
beforeEach(() => {
  vi.resetAllMocks()
  m.session.mockReturnValue({ authTime: 100 })
  m.blocked.mockReturnValue(true)
  m.pending.mockReturnValue([])
  m.enable.mockResolvedValue({ ok: true })
})
it('refuses permission failure before touching the IdP', async () => {
  m.permission.mockImplementation(() => { throw new Error('Forbidden') })
  await expect(run({ data: { username: 'alice' } })).rejects.toThrow('Forbidden')
  expect(m.enable).not.toHaveBeenCalled()
})
it('refuses missing verified authentication time', async () => {
  m.session.mockReturnValue({})
  await expect(run({ data: { username: 'alice' } })).rejects.toThrow('auth_time')
  expect(m.enable).not.toHaveBeenCalled()
})
it('refuses pending cleanup', async () => {
  m.pending.mockReturnValue(['unfinished'])
  await expect(run({ data: { username: 'alice' } })).rejects.toThrow('pendentes')
  expect(m.enable).not.toHaveBeenCalled()
})
it('preserves local block when IdP reactivation fails', async () => {
  m.enable.mockResolvedValue({ ok: false, detail: 'IdP failure' })
  await expect(run({ data: { username: 'alice' } })).rejects.toThrow('IdP failure')
  expect(m.activate).not.toHaveBeenCalled()
})
it('reactivates and audits only after the IdP accepts', async () => {
  await expect(run({ data: { username: 'alice' } })).resolves.toMatchObject({ ok: true })
  expect(m.activate).toHaveBeenCalledWith('alice')
  expect(m.audit).toHaveBeenCalledWith('POST', '/archgate/persons/alice/reactivate', 'admin', 'success')
})
