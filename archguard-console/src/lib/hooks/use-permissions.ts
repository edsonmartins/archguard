// src/lib/hooks/use-permissions.ts

import { useRouteContext } from '@tanstack/react-router'
import type { Permission } from '../auth/permissions'
import { hasPermission, hasAnyPermission } from '../auth/permissions'

export function usePermissions() {
  const { auth } = useRouteContext({ from: '/_authed' })

  return {
    groups: auth.groups,
    permissions: auth.permissions,
    can: (perm: Permission | Permission[]) =>
      hasPermission(auth.permissions, perm),
    canAny: (perms: Permission[]) =>
      hasAnyPermission(auth.permissions, perms),
    isSystemAdmin: auth.permissions.includes('system:admin'),
    isTenantAdmin: auth.groups.some(
      (g) =>
        g.endsWith('_admins') &&
        !['archguard_admins', 'idm_admins'].includes(g),
    ),
    tenants: auth.groups
      .filter((g) => g.endsWith('_admins'))
      .filter((g) => !['archguard_admins', 'idm_admins'].includes(g))
      .map((g) => g.replace(/_admins$/, '')),
  }
}
