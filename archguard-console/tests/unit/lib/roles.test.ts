import { describe, it, expect } from 'vitest'
import {
  deriveRole,
  deriveTenants,
  isTenantAdminRole,
} from '@/lib/auth/roles'

describe('deriveRole', () => {
  it('maps archguard_super_admins to SUPER_ADMIN', () => {
    expect(deriveRole(['archguard_super_admins', 'archguard_users'])).toBe(
      'SUPER_ADMIN',
    )
  })

  it('maps archguard_tenant_admins to TENANT_ADMIN', () => {
    expect(
      deriveRole(['archguard_tenant_admins', 'tenant_rio_quality']),
    ).toBe('TENANT_ADMIN')
  })

  it('maps legacy acme_admins to TENANT_ADMIN', () => {
    expect(deriveRole(['acme_admins'])).toBe('TENANT_ADMIN')
  })

  it('maps service desk groups', () => {
    expect(deriveRole(['archguard_service_desk'])).toBe('SERVICE_DESK')
    expect(deriveRole(['idm_service_desk'])).toBe('SERVICE_DESK')
  })

  it('maps operator/viewer to VIEWER', () => {
    expect(deriveRole(['archguard_users', 'tenant_rio_quality'])).toBe(
      'VIEWER',
    )
    expect(deriveRole(['archguard_viewers'])).toBe('VIEWER')
  })
})

describe('deriveTenants', () => {
  it('extracts ArchGate tenant_* groups', () => {
    expect(
      deriveTenants([
        'archguard_users',
        'tenant_rio_quality',
        'tenant_grupo_marra',
      ]),
    ).toEqual(['tenant_grupo_marra', 'tenant_rio_quality'])
  })

  it('extracts legacy {tenant}_admins', () => {
    expect(deriveTenants(['acme_admins', 'globex_admins'])).toEqual([
      'acme',
      'globex',
    ])
  })

  it('ignores platform *_admins groups as tenants', () => {
    expect(
      deriveTenants([
        'archguard_super_admins',
        'archguard_tenant_admins',
        'idm_admins',
        'tenant_rio_quality',
      ]),
    ).toEqual(['tenant_rio_quality'])
  })

  it('strips SPN domain', () => {
    expect(
      deriveTenants(['tenant_rio_quality@id.archgate.com.br']),
    ).toEqual(['tenant_rio_quality'])
  })
})

describe('isTenantAdminRole', () => {
  it('true for archguard_tenant_admins', () => {
    expect(isTenantAdminRole(['archguard_tenant_admins'])).toBe(true)
  })

  it('false for super admin', () => {
    expect(
      isTenantAdminRole(['archguard_super_admins', 'archguard_tenant_admins']),
    ).toBe(false)
  })

  it('false for plain operator', () => {
    expect(
      isTenantAdminRole(['archguard_users', 'tenant_rio_quality']),
    ).toBe(false)
  })
})
