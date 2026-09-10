import { beforeEach, expect, it, vi } from 'vitest'
const m = vi.hoisted(() => ({ current: vi.fn(), revoke: vi.fn() }))
vi.mock('@tanstack/react-start', () => ({ createServerFn: () => ({
  handler: (fn: (ctx: { data: unknown }) => unknown) => fn,
  inputValidator: (parse: (input: unknown) => unknown) => ({
    handler: (fn: (ctx: { data: unknown }) => unknown) => (ctx: { data: unknown }) => fn({ data: parse(ctx.data) }),
  }),
}) }))
vi.mock('@/server/openbao-proxy', () => ({
  revokeLease: m.revoke, getHealth: vi.fn(), getJwtConfig: vi.fn(), getSealStatus: vi.fn(),
  listAuthMethods: vi.fn(), listDbLeases: vi.fn(), listMounts: vi.fn(), listPolicies: vi.fn(),
  openbaoAddr: vi.fn(), openbaoConfigured: vi.fn(), openbaoTokenConfigured: vi.fn(), openbaoTokenKind: vi.fn(), unsealWithEnvKey: vi.fn(),
}))
vi.mock('@/server/session-guard', () => ({ requireSession: m.current, requireAnyPerm: (s: { permissions: string[] }, required: string[]) => {
  if (!required.some((permission) => s.permissions.includes(permission))) throw new Error('Forbidden')
} }))
import { revokeOpenBaoLeaseFn } from '@/server/openbao-fn'
const invoke = revokeOpenBaoLeaseFn as unknown as (ctx: { data: { lease_id: string } }) => Promise<unknown>
beforeEach(() => {
  vi.resetAllMocks()
  m.current.mockReturnValue({ permissions: ['secrets:manage'] })
})
it('denies arbitrary lease revocation to secrets managers', async () => {
  await expect(invoke({ data: { lease_id: 'database/creds/role/lease-a' } })).rejects.toThrow('Forbidden')
  expect(m.revoke).not.toHaveBeenCalled()
})
it('allows only platform administration', async () => {
  m.current.mockReturnValue({ permissions: ['system:admin'] })
  m.revoke.mockResolvedValue(undefined)
  await expect(invoke({ data: { lease_id: 'database/creds/role/lease-a' } })).resolves.toEqual({ ok: true })
  expect(m.revoke).toHaveBeenCalledWith('database/creds/role/lease-a')
})
