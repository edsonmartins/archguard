import { describe, it, expect } from 'vitest'
import {
  derivePermissions,
  hasPermission,
  hasAnyPermission,
  ALL_PERMISSIONS,
  type Permission,
} from '@/lib/auth/permissions'

describe('derivePermissions', () => {
  it('returns no permissions for an empty group list', () => {
    expect(derivePermissions([])).toEqual([])
  })

  it('elevates idm_admins to ALL_PERMISSIONS', () => {
    const perms = derivePermissions(['idm_admins'])
    expect(perms).toEqual(ALL_PERMISSIONS)
  })

  it('elevates archguard_admins to ALL_PERMISSIONS', () => {
    const perms = derivePermissions(['archguard_admins'])
    expect(perms).toEqual(ALL_PERMISSIONS)
  })

  it('grants people-admin granular perms without system:admin', () => {
    const perms = derivePermissions(['idm_people_admins'])
    expect(perms).toContain('persons:create')
    expect(perms).toContain('persons:credentials')
    expect(perms).toContain('groups:members')
    expect(perms).not.toContain('oauth2:create')
    expect(perms).not.toContain('system:admin')
  })

  it('grants oauth2-admin only oauth2 perms', () => {
    const perms = derivePermissions(['idm_oauth2_admins'])
    expect(perms).toContain('oauth2:create')
    expect(perms).toContain('oauth2:secrets')
    expect(perms).not.toContain('persons:create')
    expect(perms).not.toContain('system:admin')
  })

  it('service desk gets read-only and credential reset', () => {
    const perms = derivePermissions(['idm_service_desk'])
    expect(perms).toContain('persons:read')
    expect(perms).toContain('persons:credentials')
    expect(perms).toContain('groups:read')
    expect(perms).not.toContain('persons:create')
    expect(perms).not.toContain('persons:delete')
  })

  it('on-boarding allows person create + import without delete', () => {
    const perms = derivePermissions(['idm_people_on_boarding'])
    expect(perms).toContain('persons:create')
    expect(perms).toContain('persons:import')
    expect(perms).not.toContain('persons:delete')
  })

  it('treats arbitrary {tenant}_admins via the fallback rule', () => {
    const perms = derivePermissions(['acme_admins'])
    expect(perms).toContain('persons:read')
    expect(perms).toContain('persons:create')
    expect(perms).toContain('groups:members')
    expect(perms).toContain('audit:read')
    // Tenant admin is NOT a system admin.
    expect(perms).not.toContain('system:admin')
    expect(perms).not.toContain('oauth2:delete')
  })

  it('union of multiple groups (oauth2 + people)', () => {
    const perms = derivePermissions(['idm_oauth2_admins', 'idm_people_admins'])
    expect(perms).toContain('oauth2:create')
    expect(perms).toContain('persons:create')
    expect(perms).not.toContain('system:admin')
  })

  it('any system:admin source eclipses union', () => {
    const perms = derivePermissions(['idm_oauth2_admins', 'idm_admins'])
    expect(perms).toEqual(ALL_PERMISSIONS)
  })

  it('unknown groups produce no permissions', () => {
    expect(derivePermissions(['random_group'])).toEqual([])
  })

  it('archguard_super_admins elevates to ALL_PERMISSIONS', () => {
    expect(derivePermissions(['archguard_super_admins'])).toEqual(
      ALL_PERMISSIONS,
    )
  })

  it('archguard_tenant_admins gets tenant-admin perms without system:admin', () => {
    const perms = derivePermissions(['archguard_tenant_admins'])
    expect(perms).toContain('persons:create')
    expect(perms).toContain('groups:members')
    expect(perms).toContain('audit:read')
    expect(perms).not.toContain('system:admin')
    expect(perms).not.toContain('oauth2:delete')
  })

  it('archguard_users (operator) is read-only', () => {
    const perms = derivePermissions(['archguard_users'])
    expect(perms).toContain('persons:read')
    expect(perms).toContain('groups:read')
    expect(perms).not.toContain('persons:create')
    expect(perms).not.toContain('persons:delete')
  })

  it('archguard_viewers is read-only', () => {
    const perms = derivePermissions(['archguard_viewers', 'tenant_rio_quality'])
    expect(perms).toContain('persons:read')
    expect(perms).not.toContain('persons:create')
  })

  it('does not treat archguard_tenant_admins as {tenant}_admins fallback name', () => {
    // If mistaken as tenant "archguard_tenant", would still get tenant perms —
    // ensure explicit mapping is used and no oauth2 leak.
    const perms = derivePermissions(['archguard_tenant_admins'])
    expect(perms).not.toContain('oauth2:create')
  })
})

describe('hasPermission', () => {
  it('returns true if the user has the required permission', () => {
    expect(hasPermission(['persons:read'], 'persons:read')).toBe(true)
  })

  it('returns false if the permission is missing', () => {
    expect(hasPermission(['persons:read'], 'persons:delete')).toBe(false)
  })

  it('system:admin bypasses any required permission', () => {
    expect(hasPermission(['system:admin'], 'oauth2:delete')).toBe(true)
    expect(
      hasPermission(['system:admin'], ['persons:delete', 'oauth2:secrets']),
    ).toBe(true)
  })

  it('requires ALL when given an array', () => {
    const perms: Permission[] = ['persons:read', 'persons:create']
    expect(hasPermission(perms, ['persons:read', 'persons:create'])).toBe(true)
    expect(hasPermission(perms, ['persons:read', 'persons:delete'])).toBe(false)
  })
})

describe('hasAnyPermission', () => {
  it('returns true if any required permission is held', () => {
    expect(
      hasAnyPermission(['persons:read'], ['persons:delete', 'persons:read']),
    ).toBe(true)
  })

  it('returns false when none match', () => {
    expect(
      hasAnyPermission(['persons:read'], ['persons:delete', 'oauth2:create']),
    ).toBe(false)
  })

  it('system:admin returns true even without explicit grant', () => {
    expect(hasAnyPermission(['system:admin'], ['oauth2:delete'])).toBe(true)
  })
})
