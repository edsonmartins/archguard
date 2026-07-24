import { describe, it, expect, vi, beforeEach } from 'vitest'

const listSites = vi.fn()
const rolesForWarpgateTarget = vi.fn()

vi.mock('@/server/sites', () => ({
  listSites: (...args: unknown[]) => listSites(...args),
}))

vi.mock('@/server/warpgate-proxy', () => ({
  rolesForWarpgateTarget: (...args: unknown[]) =>
    rolesForWarpgateTarget(...args),
  bindWarpgateUserRole: vi.fn(),
  warpgateConfigured: () => true,
}))

import { resolveGrantRoles } from '@/server/lifecycle-fn'

describe('resolveGrantRoles', () => {
  beforeEach(() => {
    listSites.mockReset()
    rolesForWarpgateTarget.mockReset()
  })

  it('prefers explicit role', async () => {
    const r = await resolveGrantRoles('rio-aws-api-b', 'tenant-rio-quality')
    expect(r.roles).toEqual(['tenant-rio-quality'])
    expect(rolesForWarpgateTarget).not.toHaveBeenCalled()
  })

  it('uses Warpgate target roles when present', async () => {
    rolesForWarpgateTarget.mockResolvedValue({
      targetFound: true,
      roles: ['tenant-rio-quality'],
      detail: 'roles from WG',
    })
    const r = await resolveGrantRoles('rio-aws-api-b')
    expect(r.roles).toEqual(['tenant-rio-quality'])
  })

  it('falls back to site SoT roles', async () => {
    rolesForWarpgateTarget.mockResolvedValue({
      targetFound: true,
      roles: [],
      detail: 'no roles on object',
    })
    listSites.mockResolvedValue([
      {
        slug: 'rio_quality',
        warpgate_roles: ['tenant-rio-quality'],
        targets: [{ nome: 'rio-aws-api-b', roles: [] }],
      },
    ])
    const r = await resolveGrantRoles('rio-aws-api-b')
    expect(r.roles).toEqual(['tenant-rio-quality'])
    expect(r.detail).toMatch(/SoT site rio_quality/)
  })
})
