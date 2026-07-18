import { describe, expect, it } from 'vitest'
import type { SessionData } from '@/server/auth'
import type { Site } from '@/lib/api/types/site'
import { filterSitesByTenant } from '@/server/session-guard'

function session(
  partial: Partial<SessionData> & {
    groups: string[]
    permissions?: SessionData['permissions']
  },
): SessionData {
  return {
    isAuthenticated: true,
    isAdmin: false,
    user: { id: 'u', name: 'u', email: 'u@x', displayName: 'u' },
    expiresAt: Date.now() + 60_000,
    ...partial,
    groups: partial.groups,
    permissions: partial.permissions || [],
  }
}

const sites: Site[] = [
  {
    slug: 'rio',
    cliente: 'Rio',
    tenant_group: 'tenant_rio_quality',
    ambiente: 'lab',
    tipo: 'tunnel_agent',
    stack: 'lab_overlay',
    connector_id: 'c1',
    subnets: [],
    stack_meta: {},
    targets: [],
    warpgate_roles: [],
    notas: '',
    inventariado: false,
    connector_deployed: false,
    smoke_operador: false,
    updated_at: '',
    updated_by: null,
  },
  {
    slug: 'marra',
    cliente: 'Marra',
    tenant_group: 'tenant_grupo_marra',
    ambiente: 'lab',
    tipo: 'tunnel_agent',
    stack: 'lab_overlay',
    connector_id: 'c2',
    subnets: [],
    stack_meta: {},
    targets: [],
    warpgate_roles: [],
    notas: '',
    inventariado: false,
    connector_deployed: false,
    smoke_operador: false,
    updated_at: '',
    updated_by: null,
  },
]

describe('filterSitesByTenant', () => {
  it('system:admin sees all sites', () => {
    const s = session({
      groups: ['archguard_super_admins'],
      permissions: ['system:admin'],
    })
    expect(filterSitesByTenant(sites, s)).toHaveLength(2)
  })

  it('deny-by-default when principal has no tenants', () => {
    const s = session({
      groups: ['archguard_viewers'],
      permissions: ['sites:read'],
    })
    expect(filterSitesByTenant(sites, s)).toEqual([])
  })

  it('filters to membership tenants only', () => {
    const s = session({
      groups: ['tenant_rio_quality', 'archguard_users'],
      permissions: ['sites:read'],
    })
    const filtered = filterSitesByTenant(sites, s)
    expect(filtered.map((x) => x.slug)).toEqual(['rio'])
  })
})
