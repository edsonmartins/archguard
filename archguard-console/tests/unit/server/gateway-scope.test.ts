import { describe, expect, it } from 'vitest'
import {
  filterByRoleNames,
  filterByTargetNames,
  type GatewayTenantScope,
} from '@/server/gateway-scope'

describe('gateway scope filters', () => {
  it('unrestricted passes all', () => {
    const scope: GatewayTenantScope = {
      unrestricted: true,
      targetNames: new Set(),
      roleNames: new Set(),
      siteSlugs: [],
    }
    const items = [{ name: 'a' }, { name: 'b' }]
    expect(filterByTargetNames(items, scope)).toEqual(items)
    expect(filterByRoleNames(items, scope)).toEqual(items)
  })

  it('deny-by-default when allow-list empty', () => {
    const scope: GatewayTenantScope = {
      unrestricted: false,
      targetNames: new Set(),
      roleNames: new Set(),
      siteSlugs: [],
    }
    expect(filterByTargetNames([{ name: 'x' }], scope)).toEqual([])
    expect(filterByRoleNames([{ name: 'y' }], scope)).toEqual([])
  })

  it('filters to declared names only', () => {
    const scope: GatewayTenantScope = {
      unrestricted: false,
      targetNames: new Set(['rio-lab-ssh']),
      roleNames: new Set(['tenant-rio-quality']),
      siteSlugs: ['rio_quality_lab'],
    }
    expect(
      filterByTargetNames(
        [{ name: 'rio-lab-ssh' }, { name: 'marra-lab-ssh' }],
        scope,
      ).map((x) => x.name),
    ).toEqual(['rio-lab-ssh'])
    expect(
      filterByRoleNames(
        [{ name: 'tenant-rio-quality' }, { name: 'tenant-grupo-marra' }],
        scope,
      ).map((x) => x.name),
    ).toEqual(['tenant-rio-quality'])
  })
})
